package httpserver

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var addrRe = regexp.MustCompile("^(.*):([0-9]+)$")

type ResponseLogger struct {
	w http.ResponseWriter
	r *http.Request
	start time.Time
	bytesWritten int
	statusCode int
}

func NewResponseLogger(w http.ResponseWriter, r *http.Request) *ResponseLogger {
	return &ResponseLogger{
		w: w,
		r: r,
		start: time.Now(),
		bytesWritten: 0,
		statusCode: 0,
	}
}

func (rl *ResponseLogger) ip() string {
	if strings.HasPrefix(rl.r.RemoteAddr, "[::1]:") {
		return "127.0.0.1"
	}
	parts := addrRe.FindStringSubmatch(rl.r.RemoteAddr)
	if parts != nil && len(parts) > 1 {
		return parts[1]
	}
	return rl.r.RemoteAddr
}

func (rl *ResponseLogger) WriteLog(w io.Writer) {
	dt := time.Now().Sub(rl.start)
	format := `%s [%s] "%s %s" %d %d %.3f "%s" "%s"` + "\n"
	args := []interface{}{
		rl.ip(),
		rl.start.Format("2006-01-02 15:04:05 -0700"),
		//rl.start.Format("02/Jan/2006:15:04:05 -0700"),
		rl.r.Method,
		rl.r.URL.String(),
		rl.statusCode,
		rl.bytesWritten,
		float64(dt) / 1.0e6,
		rl.r.Referer(),
		rl.r.UserAgent(),
	}
	w.Write([]byte(fmt.Sprintf(format, args...)))
}

func (rl *ResponseLogger) Header() http.Header {
	return rl.w.Header()
}

func (rl *ResponseLogger) WriteHeader(statusCode int) {
	rl.statusCode = statusCode
	rl.w.WriteHeader(statusCode)
}

func (rl *ResponseLogger) Write(data []byte) (int, error) {
	if rl.statusCode == 0 {
		rl.statusCode = http.StatusOK
	}
	n, err := rl.w.Write(data)
	rl.bytesWritten += n
	return n, err
}

func (rl *ResponseLogger) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := rl.w.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("webserver doesn't support hijacking")
	}
	return hj.Hijack()
}
