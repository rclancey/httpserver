package httpserver

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	mutex *sync.Mutex
	reg *prometheus.Registry
	collectors map[string]prometheus.Collector
	labels map[string][]string
}

type MetricsWriter struct {
	w http.ResponseWriter
	status int
	bytesWritten int
	startTime time.Time
	firstWriteTime time.Time
	lastWriteTime time.Time
}

func NewMetricsWriter(w http.ResponseWriter) *MetricsWriter {
	return &MetricsWriter{
		w: w,
		startTime: time.Now(),
	}
}

func (mw *MetricsWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hw, ok := mw.w.(http.Hijacker)
	if ok {
		return hw.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter %T doesn't support hijacking", mw.w)
}

func (mw *MetricsWriter) Header() http.Header {
	return mw.w.Header()
}

func (mw *MetricsWriter) Write(data []byte) (int, error) {
	if mw.firstWriteTime.IsZero() {
		mw.firstWriteTime = time.Now()
	}
	n, err := mw.w.Write(data)
	mw.lastWriteTime = time.Now()
	mw.bytesWritten += n
	return n, err
}

func (mw *MetricsWriter) WriteHeader(status int) {
	mw.status = status
	mw.w.WriteHeader(status)
}

func (mw *MetricsWriter) Measure(route string) {
	m := metricsSingleton
	labels := map[string]string{
		"route": route,
		"status": strconv.Itoa(mw.status),
	}
	m.Count("http_request_count", labels)
	m.Summarize("http_response_size", labels, float64(mw.bytesWritten))
	if !mw.lastWriteTime.IsZero() {
		m.Summarize("http_response_first_write", labels, mw.firstWriteTime.Sub(mw.startTime).Seconds())
		m.Summarize("http_response_time", labels, mw.lastWriteTime.Sub(mw.startTime).Seconds())
	}
}

var metricsSingleton = &Metrics{
	mutex: &sync.Mutex{},
	reg: prometheus.NewRegistry(),
	collectors: map[string]prometheus.Collector{},
	labels: map[string][]string{},
}

func init() {
	start := time.Now()
	go func() {
		ticker := time.NewTicker(time.Second)
		for {
			<-ticker.C
			Measure("uptime", nil, time.Since(start).Seconds())
		}
	}()
	metricsSingleton.reg.Register(collectors.NewGoCollector())
	metricsSingleton.reg.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{Registry: m.reg}).(http.HandlerFunc)
}

func (m *Metrics) AttachEndpoint(router Router) {
	router.GET("/metrics", m.Handler())
}

func Measure(name string, labels map[string]string, value float64) {
	m := metricsSingleton
	m.Measure(name, labels, value)
}

func (m *Metrics) Measure(name string, labels map[string]string, value float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	collector, ok := m.collectors[name]
	if !ok {
		var labelKeys []string
		if labels == nil || len(labels) == 0 {
			collector = prometheus.NewGauge(prometheus.GaugeOpts{Name: name})
		} else {
			for k := range labels {
				labelKeys = append(labelKeys, k)
			}
			collector = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name}, labelKeys)
		}
		m.collectors[name] = collector
		m.labels[name] = labelKeys
		m.reg.Register(collector)
	}
	labelKeys := m.labels[name]
	if labelKeys == nil {
		gauge, ok := collector.(prometheus.Gauge)
		if !ok {
			return
		}
		gauge.Set(value)
	} else {
		gauge, ok := collector.(*prometheus.GaugeVec)
		if !ok {
			return
		}
		labelVals := make([]string, len(labelKeys))
		for i, k := range labelKeys {
			labelVals[i] = labels[k]
		}
		gauge.WithLabelValues(labelVals...).Set(value)
	}
}

func Summarize(name string, labels map[string]string, value float64) {
	m := metricsSingleton
	m.Summarize(name, labels, value)
}

func (m *Metrics) Summarize(name string, labels map[string]string, value float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	collector, ok := m.collectors[name]
	if !ok {
		var labelKeys []string
		if labels == nil || len(labels) == 0 {
			collector = prometheus.NewSummary(prometheus.SummaryOpts{
				Name: name,
				MaxAge: 10 * time.Minute,
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			})
		} else {
			for k := range labels {
				labelKeys = append(labelKeys, k)
			}
			collector = prometheus.NewSummaryVec(prometheus.SummaryOpts{
				Name: name,
				MaxAge: 10 * time.Minute,
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			}, labelKeys)
		}
		m.collectors[name] = collector
		m.labels[name] = labelKeys
		m.reg.Register(collector)
	}
	labelKeys := m.labels[name]
	if labelKeys == nil {
		summary, ok := collector.(prometheus.Summary)
		if !ok {
			return
		}
		summary.Observe(value)
	} else {
		summary, ok := collector.(*prometheus.SummaryVec)
		if !ok {
			return
		}
		labelVals := make([]string, len(labelKeys))
		for i, k := range labelKeys {
			labelVals[i] = labels[k]
		}
		summary.WithLabelValues(labelVals...).Observe(value)
	}
}

func Increment(name string, labels map[string]string, value float64) {
	m := metricsSingleton
	m.Increment(name, labels, value)
}

func (m *Metrics) Increment(name string, labels map[string]string, value float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	collector, ok := m.collectors[name]
	if !ok {
		var labelKeys []string
		if labels == nil || len(labels) == 0 {
			collector = prometheus.NewCounter(prometheus.CounterOpts{Name: name})
		} else {
			for k := range labels {
				labelKeys = append(labelKeys, k)
			}
			collector = prometheus.NewCounterVec(prometheus.CounterOpts{Name: name}, labelKeys)
		}
		m.collectors[name] = collector
		m.labels[name] = labelKeys
		m.reg.Register(collector)
	}
	labelKeys := m.labels[name]
	if labelKeys == nil {
		counter, ok := collector.(prometheus.Counter)
		if !ok {
			return
		}
		counter.Add(value)
	} else {
		counter, ok := collector.(*prometheus.CounterVec)
		if !ok {
			return
		}
		labelVals := make([]string, len(labelKeys))
		for i, k := range labelKeys {
			labelVals[i] = labels[k]
		}
		counter.WithLabelValues(labelVals...).Add(value)
	}
}

func Count(name string, labels map[string]string) {
	m := metricsSingleton
	m.Count(name, labels)
}

func (m *Metrics) Count(name string, labels map[string]string) {
	m.Increment(name, labels, 1.0)
}
