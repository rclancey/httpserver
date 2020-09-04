package httpserver

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var fwdRe = regexp.MustCompile(`([\s,;=]+)=([^\s,;=]+|"[^"]*")([;,$])`)

func parseForwarded(h string) [][]string {
	if h == "" {
		return [][]string{}
	}
	fwd := [][]string{[]string{}}
	idx := 0
	ms := fwdRe.FindAllStringSubmatch(strings.TrimSpace(h), -1)
	for _, m := range ms {
		fwd[idx] = append(fwd[idx], fmt.Sprintf("%s=%s", m[0], m[1]))
		if m[2] == "," {
			fwd = append(fwd, []string{})
			idx += 1
		}
	}
	if len(fwd[idx]) == 0 {
		fwd = fwd[:idx - 1]
	}
	return fwd
}

func formatForwarded(fwd [][]string) string {
	parts := make([]string, len(fwd))
	for i, part := range fwd {
		parts[i] = strings.Join(part, ";")
	}
	return strings.Join(parts, ", ")
}

func parseAddr(addr string) string {
	parts := strings.Split(addr, ":")
	parts = parts[:len(parts) - 1]
	return strings.Join(parts, ":")
}

func Proxy(w http.ResponseWriter, req *http.Request, proxyUrl string) {
	client := &http.Client{ Timeout: 30 * time.Second }
	defer client.CloseIdleConnections()
	preq, err := http.NewRequest(req.Method, proxyUrl, req.Body)
	if err != nil {
		sendError(w, req, BadRequest.Wrap(err, "Invalid downstream server"))
		return
	}
	fwd := parseForwarded(req.Header.Get("Forwarded"))
	ip := parseAddr(req.RemoteAddr)
	fwd = append(fwd, []string{
		fmt.Sprintf(`for="%s"`, ip),
		fmt.Sprintf(`host="%s"`, req.Host),
		fmt.Sprintf(`proto=%s`, req.URL.Scheme),
	})
	preq.Header.Set("X-Forwarded-Host", req.Host)
	preq.Header.Set("X-Forwarded-Proto", req.URL.Scheme)
	preq.Header.Set("X-Real-IP", ip)
	for k, vs := range req.Header {
		switch k {
		case "Host":
		case "Forwarded":
		default:
			preq.Header[k] = vs
		}
	}
	xff := preq.Header.Get("X-Forwarded-For")
	if xff == "" {
		preq.Header.Set("X-Forwarded-For", ip)
	} else {
		preq.Header.Set("X-Forwarded-For", fmt.Sprintf("%s, %s", xff, ip))
	}
	preq.Header.Set("Forwarded", formatForwarded(fwd))
	res, err := client.Do(preq)
	if err != nil {
		sendError(w, req, BadGateway.Wrap(err, "Downstream server error"))
		return
	}
	wh := w.Header()
	for k, vs := range res.Header {
		wh[k] = vs
	}
	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)
	res.Body.Close()
}

