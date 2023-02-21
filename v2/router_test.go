package httpserver

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }
type RouteSuite struct {}

var _ = Suite(&RouteSuite{})

func (s *RouteSuite) TestRoute(c *C) {
	r := &Route{Method: http.MethodPatch, Path: "/path/:to/:rsrc"}
	c.Check(r.String(), Equals, "PATCH /path/:to/:rsrc")
	params := map[string]string{
		"rsrc": "foo/bar",
	}
	c.Check(r.URL(params), Equals, "/path/undefined/foo%2Fbar")
}

func (s *RouteSuite) TestValidateRouter(c *C) {
	r := NewRouter()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.GET("/foo/bar", h)
	r.GET("/foo/:id", h)
	r.GET("/foo/:name/stuff", h)
	r.POST("/foo/:name", h)
	err := ValidateRouter(r)
	c.Check(err, IsNil)
	r.GET("/foo/:id/stuff", h)
	err = ValidateRouter(r)
	c.Check(err, ErrorMatches, "^.*ambiguous routes.*$")
}

type MockResponse struct {
	header http.Header
	status int
	data *bytes.Buffer
}

func NewMockResponse() *MockResponse {
	return &MockResponse{
		header: http.Header{},
		status: -1,
		data: bytes.NewBuffer([]byte{}),
	}
}

func (w *MockResponse) Header() http.Header {
	return w.header
}

func (w *MockResponse) WriteHeader(status int) {
	if status > 0 && w.status < 0 {
		w.status = status
	}
}

func (w *MockResponse) Write(data []byte) (int, error) {
	if w.status < 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.data.Write(data)
}

func (w *MockResponse) Status() int {
	return w.status
}

func (w *MockResponse) Data() []byte {
	return w.data.Bytes()
}

type ctxKey string

func NewMockRequest(path string, n int) *http.Request {
	ctx := context.WithValue(context.Background(), ctxKey("t"), n)
	return httptest.NewRequest(http.MethodGet, path, nil).Clone(ctx)
}

func NamedHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, isa := r.Context().Value(ctxKey("t")).(int)
		w.WriteHeader(http.StatusOK)
		if isa {
			w.Write([]byte(fmt.Sprintf("%s %d", name, n)))
		} else {
			w.Write([]byte(name))
		}
	})
}

func (s *RouteSuite) TestLookup(c *C) {
	type test struct {
		Method string
		Path string
		Handler bool
		Params map[string]string
		Output string
	}
	r := NewRouter()
	r.GET("/foo/:get/bar", NamedHandler("lennon"))
	r.POST("/foo/baz/:post", NamedHandler("mccartney"))
	r.PUT("/foo/:put/", NamedHandler("harrison"))
	r.PATCH("/foo/:patch", NamedHandler("starr"))
	r.DELETE("/foo/:delete", NamedHandler("jagger"))
	r.Compile([]Middleware{})
	exp := []*test{
		&test{http.MethodGet, "/foo/john", false, nil, ""},
		&test{http.MethodGet, "/foo/john/bar", true, map[string]string{"get": "john", "route": "/foo/:get/bar"}, "lennon 0"},
		&test{http.MethodPost, "/foo/baz/paul", true, map[string]string{"post": "paul", "route": "/foo/baz/:post"}, "mccartney 0"},
		&test{http.MethodPut, "/foo/george", true, map[string]string{"put": "george", "route": "/foo/:put"}, "harrison 0"},
		&test{http.MethodPut, "/foo/george/washington", true, map[string]string{"put": "george", "filepath": "washington", "route": "/foo/:put"}, "harrison 0"},
		&test{http.MethodPatch, "/foo/ringo/beatles", true, map[string]string{"patch": "ringo", "filepath": "beatles", "route": "/foo/:patch"}, "starr 0"},
		&test{http.MethodPatch, "/foo/ringo", true, map[string]string{"patch": "ringo", "route": "/foo/:patch"}, "starr 0"},
		&test{http.MethodDelete, "/foo/mick", true, map[string]string{"delete": "mick", "route": "/foo/:delete"}, "jagger 0"},
	}
	for i, t := range exp {
		c.Log("lookup test", i)
		req := NewMockRequest(t.Path, 0)
		w := NewMockResponse()
		h, params := r.LookupPath(t.Method, t.Path)
		if t.Handler {
			c.Check(h, NotNil)
			for k, v := range t.Params {
				c.Check(params[k], Equals, v)
			}
			for k := range params {
				_, ok := t.Params[k]
				c.Check(ok, Equals, true)
			}
			if h != nil {
				h.ServeHTTP(w, req)
				c.Check(string(w.Data()), Equals, t.Output)
			}
		} else {
			c.Check(h, IsNil)
		}
	}
}

func (s *RouteSuite) TestCompile(c *C) {
	mw1 := func(h http.Handler) http.Handler {
		c.Log("applying middleware mw1")
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Log("mw1", r.URL.Path)
			n, _ := r.Context().Value(ctxKey("t")).(int)
			n *= 2
			r = r.Clone(context.WithValue(r.Context(), ctxKey("t"), n))
			h.ServeHTTP(w, r)
		})
	}
	mw2 := func(h http.Handler) http.Handler {
		c.Log("applying middleware mw2")
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Log("mw1", r.URL.Path)
			n, _ := r.Context().Value(ctxKey("t")).(int)
			n += 1
			r = r.Clone(context.WithValue(r.Context(), ctxKey("t"), n))
			h.ServeHTTP(w, r)
		})
	}
	r := NewRouter()
	r.GET("/foo/bar", NamedHandler("bar"))
	r.GET("/baz/qux", NamedHandler("qux"))
	r.Prefix("/foo").Use(mw1)
	r.Prefix("/foo").Use(mw2)
	r.Prefix("/baz").Use(mw2)
	r.GET("/baz/blah", NamedHandler("blah"))
	r.Compile([]Middleware{})
	r.Prefix("/baz").Use(mw1)
	err := ValidateRouter(r)
	c.Check(err, IsNil)
	r.Compile([]Middleware{})
	exp := map[string]string{
		"/foo/bar": "bar 11",
		"/baz/qux": "qux 12",
		"/baz/blah": "blah 12",
	}
	for k, v := range exp {
		c.Log("testing route", k)
		req := NewMockRequest(k, 5)
		w := NewMockResponse()
		pth := strings.Split(strings.TrimPrefix(path.Clean(k), "/"), "/")
		h, _ := r.Lookup(http.MethodGet, pth)
		c.Check(h, NotNil)
		if h != nil {
			h.ServeHTTP(w, req)
			c.Check(string(w.Data()), Equals, v)
		}
	}
}
