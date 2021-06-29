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

type compressor interface {
	io.WriteCloser
	Flush() error
}

func CompressMiddleware(handler http.Handler) http.Handler {
	f := func(w http.ResponseWriter, r *http.Request) {
		crw := NewCompressResponseWriter(w, r)
		handler.ServeHTTP(crw, r)
		if crw.cw != nil {
			crw.cw.Close()
		}
	}
	return http.HandlerFunc(f)
}

type CompressResponseWriter struct {
	w http.ResponseWriter
	compressor func() compressor
	cw compressor
}

func NewCompressResponseWriter(w http.ResponseWriter, r *http.Request) *CompressResponseWriter {
	crw := &CompressResponseWriter{w: w, cw: nil}
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
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Add("Vary", "Accept-Encoding")
				return gzip.NewWriter(w.w)
			}
		case "deflate":
			return func() compressor {
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
	if h.Get("Content-Length") != "" {
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

func (w *CompressResponseWriter) WriteHeader(statusCode int) {
	if statusCode == http.StatusOK && w.canCompress() {
		w.cw = w.compressor()
	}
	w.w.WriteHeader(statusCode)
}

func (w *CompressResponseWriter) Write(data []byte) (int, error) {
	if w.cw != nil {
		return w.cw.Write(data)
	}
	return w.w.Write(data)
}

func (w *CompressResponseWriter) Close() error {
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
		return nil, nil, errors.New("websaerver doesn't support hijacking")
	}
	return hj.Hijack()
}

