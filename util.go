package httpserver

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclancey/logging"
)

func sendError(w http.ResponseWriter, req *http.Request, err error) {
	herr, isa := err.(HTTPError)
	if !isa {
		herr = InternalServerError.Wrap(err, "")
	}
	if herr.StatusCode() >= 500 {
		log := logging.FromContext(req.Context())
		log.Error(herr.Cause())
	}
	w.WriteHeader(herr.StatusCode())
	if herr.StatusCode() >= 400 {
		w.Write([]byte(herr.Message()))
	}
}

func SendJSON(w http.ResponseWriter, obj interface{}) {
	data, err := json.Marshal(obj)
	if err != nil {
		sendError(w, nil, InternalServerError.Wrap(err, "Error serializing data to JSON"))
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func ReadJSON(req *http.Request, target interface{}) error {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return BadRequest.Wrap(err, "Failed to read request payload")
	}
	err = json.Unmarshal(body, target)
	if err != nil {
		log.Println(err)
		return BadRequest.Wrap(err, "Malformed JSON input")
	}
	return nil
}

func EnsureDir(fn string) error {
	dn := filepath.Dir(fn)
	st, err := os.Stat(dn)
	if err != nil {
		if os.IsNotExist(errors.Cause(err)) {
			return os.MkdirAll(dn, 0775)
		}
		return errors.Wrap(err, "can't stat directory " + dn)
	}
	if !st.IsDir() {
		return os.ErrExist
	}
	return nil
}

func CopyToFile(src io.Reader, fn string, overwrite bool) (string, error) {
	var dst *os.File
	var err error
	if strings.HasPrefix(fn, "*.") {
		dst, err = ioutil.TempFile("", fn)
		if err != nil {
			return "", errors.Wrap(err, "can't create tempfile " + fn)
		}
		fn = dst.Name()
	} else {
		err := EnsureDir(fn)
		if err != nil {
			return "", errors.Wrap(err, "can't ensure directory for " + fn)
		}
		st, err := os.Stat(fn)
		if err == nil {
			if !overwrite {
				return "", os.ErrExist
			}
			if st.IsDir() {
				return "", os.ErrExist
			}
		}
		dst, err = os.Create(fn)
		if err != nil {
			return "", errors.Wrap(err, "can't create destination file " + fn)
		}
	}
	defer dst.Close()
	chunk := make([]byte, 8192)
	var rn, wn, start int
	for {
		rn, err = src.Read(chunk)
		if err != nil {
			if err == io.EOF {
				return fn, nil
			}
			return fn, errors.Wrap(err, "can't read source")
		}
		start = 0
		for start < rn {
			wn, err = dst.Write(chunk[start:rn])
			if err != nil {
				return fn, errors.Wrap(err, "can't write to destination")
			}
			start += wn
		}
	}
	return fn, nil
}

func QueryScan(req *http.Request, obj interface{}) error {
	qs := req.URL.Query()
	rv := reflect.ValueOf(obj)
	if rv.Kind() != reflect.Ptr {
		return errors.New("receiver is not a pointer")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return errors.New("receiver is not a struct")
	}
	rt := rv.Type()
	n := rt.NumField()
	for i := 0; i < n; i++ {
		rf := rt.Field(i)
		name := rf.Tag.Get("url")
		if name == "" {
			name = strings.ToLower(rf.Name)
		}
		ss, ok := qs[name]
		if !ok {
			continue
		}
		var s string
		if len(ss) > 0 {
			s = ss[0]
		} else {
			s = ""
		}
		var v reflect.Value
		ft := rf.Type
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
			pv := reflect.New(ft)
			rv.Field(i).Set(pv)
			v = pv.Elem()
		} else {
			v = rv.Field(i)
		}
		switch ft.Kind() {
		case reflect.String:
			v.SetString(s)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return BadRequest.Wrapf(err, "%s param %s not an unsigned integer", name, s)
			}
			v.SetUint(iv)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			iv, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return BadRequest.Wrapf(err, "%s param %s not an integer", name, s)
			}
			v.SetInt(iv)
		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return BadRequest.Wrapf(err, "%s param %s not a number", name, s)
			}
			v.SetFloat(fv)
		case reflect.Bool:
			bv, err := strconv.ParseBool(s)
			if err != nil {
				return BadRequest.Wrapf(err, "%s param %s not a boolean", name, s)
			}
			v.SetBool(bv)
		default:
			if ft == reflect.TypeOf(time.Time{}) {
				t, err := time.Parse("2006-01-02T15:04:05MST", s)
				if err != nil {
					return BadRequest.Wrapf(err, "%s param %s not a properly formatted time stamp", name, s)
				}
				v.Set(reflect.ValueOf(t))
			} else {
				return InternalServerError.Wrapf(nil, "bad url field %s", name)
			}
		}
	}
	return nil
}

func Forwarded(r *http.Request) []map[string]string {
	fwd := r.Header.Get("Forwarded")
	if fwd == "" {
		return nil
	}
	parts := strings.Split(fwd, ",")
	ms := make([]map[string]string, len(parts))
	for i, part := range parts {
		m := map[string]string{}
		for _, pairs := range strings.Split(part, ";") {
			pair := strings.SplitN(pairs, "=", 2)
			if len(pair) != 2 {
				continue
			}
			m[strings.ToLower(pair[0])] = pair[1]
		}
		ms[i] = m
	}
	return ms
}

func ExternalURL(r *http.Request) *url.URL {
	ux := *r.URL
	u := &ux
	if u.Scheme == "" {
		if r.TLS != nil {
			u.Scheme = "https"
		} else {
			u.Scheme = "http"
		}
	}
	if u.Host == "" {
		u.Host = r.Header.Get("Host")
		if u.Host == "" {
			u.Host = r.Host
		}
	}
	fwds := Forwarded(r)
	if fwds != nil && len(fwds) > 0 {
		fwd := fwds[len(fwds) - 1]
		host := fwd["host"]
		if host != "" {
			u.Host = host
		}
		scheme := strings.ToLower(fwd["proto"])
		if scheme != "" {
			u.Scheme = scheme
		}
	} else {
		xfh := r.Header.Get("X-Forwarded-Host")
		if xfh != "" {
			parts := strings.Split(xfh, ",")
			u.Host = strings.TrimSpace(parts[len(parts) - 1])
		}
		xfp := r.Header.Get("X-Forwarded-Proto")
		if xfp != "" {
			parts := strings.Split(xfp, ",")
			u.Scheme = strings.TrimSpace(parts[len(parts) - 1])
		}
	}
	return u
}
