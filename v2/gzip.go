package httpserver

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

const (
	MinCompressSize = 2048
)

type compressor interface {
	io.WriteCloser
	Flush() error
}

func CompressMiddleware(handler http.Handler) http.Handler {
	f := func(w http.ResponseWriter, r *http.Request) {
		crw := NewCompressResponseWriter(w, r)
		handler.ServeHTTP(crw, r)
		crw.Close()
	}
	return http.HandlerFunc(f)
}

type CompressResponseWriter struct {
	headerWritten bool
	statusCode int
	w http.ResponseWriter
	buf []byte
	compressor func() compressor
	cw compressor
}

func NewCompressResponseWriter(w http.ResponseWriter, r *http.Request) *CompressResponseWriter {
	crw := &CompressResponseWriter{
		headerWritten: false,
		statusCode: 0,
		w: w,
		buf: []byte{},
		cw: nil,
	}
	crw.compressor = crw.getCompressor(r)
	return crw
}

func (w *CompressResponseWriter) getCompressor(r *http.Request) func() compressor {
	if r.Header.Get("Range") != "" {
		return nil
	}
	if strings.ToLower(strings.TrimSpace(r.Header.Get("Connection"))) == "upgrade" {
		return nil
	}
	encs := strings.Split(r.Header.Get("Accept-Encoding"), ",")
	for _, enc := range encs {
		switch strings.ToLower(strings.TrimSpace(enc)) {
		case "gzip":
			return func() compressor {
				w.Header().Del("Content-Length")
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Add("Vary", "Accept-Encoding")
				return gzip.NewWriter(w.w)
			}
		case "deflate":
			return func() compressor {
				w.Header().Del("Content-Length")
				w.Header().Set("Content-Encoding", "deflate")
				w.Header().Add("Vary", "Accept-Encoding")
				cw, err := flate.NewWriter(w.w, flate.DefaultCompression)
				if err == nil {
					return cw
				}
				return nil
			}
		}
	}
	return nil
}

func (w *CompressResponseWriter) canCompress() bool {
	if w.compressor == nil {
		return false
	}
	h := w.w.Header()
	if h.Get("Content-Encoding") != "" {
		return false
	}
	ct := strings.Split(h.Get("Content-Type"), ";")[0]
	if strings.HasPrefix(ct, "text/") {
		return true
	}
	switch ct {
	case "application/javascript":
		return true
	case "application/json":
		return true
	case "application/manifest+json":
		return true
	case "image/svg+xml":
		return true
	}
	return false
}

func (w *CompressResponseWriter) Header() http.Header {
	return w.w.Header()
}

func (w *CompressResponseWriter) writeHeader() (int, error) {
	if !w.headerWritten {
		if w.statusCode == 0 {
			w.statusCode = http.StatusOK
		}
		if w.cw == nil && w.canCompress() {
			w.cw = w.compressor()
		}
		w.w.WriteHeader(w.statusCode)
		w.headerWritten = true
	}
	if w.buf != nil && len(w.buf) > 0 {
		return w.write(w.buf)
	}
	return 0, nil
}

func (w *CompressResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	if statusCode != http.StatusOK || !w.canCompress() {
		w.writeHeader()
	}
}

func (w *CompressResponseWriter) write(data []byte) (int, error) {
	if w.cw == nil {
		return w.w.Write(data)
	}
	return w.cw.Write(data)
}

func (w *CompressResponseWriter) Write(data []byte) (int, error) {
	if w.headerWritten {
		return w.write(data)
	}
	if len(w.buf) + len(data) <= MinCompressSize {
		w.buf = append(w.buf, data...)
		return len(data), nil
	}
	_, err := w.writeHeader()
	if err != nil {
		return 0, err
	}
	return w.write(data)
}

func (w *CompressResponseWriter) Close() error {
	if !w.headerWritten {
		_, err := w.writeHeader()
		if err != nil {
			return err
		}
	}
	if w.cw != nil {
		err := w.cw.Close()
		if err != nil {
			return err
		}
	}
	c, isa := w.w.(io.Closer)
	if isa {
		return c.Close()
	}
	return nil
}

func (w *CompressResponseWriter) Flush() error {
	if !w.headerWritten {
		_, err := w.writeHeader()
		if err != nil {
			return err
		}
	}
	if w.cw != nil {
		err := w.cw.Flush()
		if err != nil {
			return err
		}
	}
	f, isa := w.w.(http.Flusher)
	if isa {
		f.Flush()
	}
	return nil
}

func (w *CompressResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.w.(http.Hijacker)
	if !ok {
		return nil, nil, errors.Errorf("compressor child response writer (%T) doesn't support hijacking", w.w)
	}
	return hj.Hijack()
}

