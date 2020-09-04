package httpserver

import (
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/pkg/errors"
)

type Route struct {
	Method string
	Path string
}

func (r *Route) String() string {
	return r.Method + " " + r.Path
}

func (r *Route) URL(params map[string]string) string {
	parts := strings.Split(strings.TrimPrefix(path.Clean(r.Path), "/"), "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			param, ok := params[part[1:]]
			if ok {
				parts[i] = url.PathEscape(param)
			} else {
				parts[i] = "undefined"
			}
		} else {
			parts[i] = url.PathEscape(part)
		}
	}
	return "/" + strings.Join(parts, "/")
}

type Router interface {
	Use(mw Middleware)
	Prefix(path string) Router
	LookupPath(method, path string) (http.Handler, map[string]string)
	Lookup(method string, path []string) (http.Handler, map[string]string)
	Handle(method, path string, handler http.Handler) error
	GET(path string, handler http.Handler) error
	POST(path string, handler http.Handler) error
	PUT(path string, handler http.Handler) error
	PATCH(path string, handler http.Handler) error
	DELETE(path string, handler http.Handler) error
	OPTIONS(path string, handler http.Handler) error
	Routes() []*Route
	Compile(parentMiddlewares []Middleware)
}

func ValidateRouter(r Router) error {
	log.Println("validate")
	seen := map[string]*Route{}
	for _, route := range r.Routes() {
		params := map[string]string{}
		parts := strings.Split(route.Path, "/")
		for _, part := range parts {
			if strings.HasPrefix(part, ":") {
				params[part[1:]] = ":PARAM:"
			}
		}
		id := route.Method + " " + route.URL(params)
		dup, ok := seen[id]
		if ok {
			return errors.Errorf("ambiguous routes: %s, %s", route, dup)
		}
		log.Println("route:", id)
		seen[id] = route
	}
	return nil
}

type prefixRouter struct {
	middlewares []Middleware
	staticRoutes map[string]Router
	paramRoutes map[string]Router
	handlers map[string]http.Handler
	compiled map[string]http.Handler
}

func NewRouter() Router {
	return &prefixRouter{
		middlewares: []Middleware{},
		staticRoutes: map[string]Router{},
		paramRoutes: map[string]Router{},
		handlers: map[string]http.Handler{},
	}
}

func (pr *prefixRouter) Compile(mws []Middleware) {
	n := len(mws)
	mymws := make([]Middleware, n + len(pr.middlewares))
	for i, mw := range mws {
		mymws[i] = mw
	}
	for i, mw := range pr.middlewares {
		mymws[i + n] = mw
	}
	n = len(mymws)
	handlers := map[string]http.Handler{}
	for method, handler := range pr.handlers {
		h := handler
		for i := n - 1; i >= 0; i-- {
			h = mymws[i](h)
		}
		handlers[method] = h
	}
	pr.compiled = handlers
	for _, sub := range pr.staticRoutes {
		sub.Compile(mymws)
	}
	for _, sub := range pr.paramRoutes {
		sub.Compile(mymws)
	}
}

func (pr *prefixRouter) Routes() []*Route {
	routes := []*Route{}
	for method := range pr.handlers {
		routes = append(routes, &Route{Method: method, Path: ""})
	}
	for prefix, sub := range pr.staticRoutes {
		for _, route := range sub.Routes() {
			route.Path = "/" + url.PathEscape(prefix) + route.Path
			routes = append(routes, route)
		}
	}
	for param, sub := range pr.paramRoutes {
		for _, route := range sub.Routes() {
			route.Path = "/:" + param + route.Path
			routes = append(routes, route)
		}
	}
	return routes
}

func (pr *prefixRouter) LookupPath(method, pth string) (http.Handler, map[string]string) {
	parts := strings.Split(strings.TrimPrefix(path.Clean(pth), "/"), "/")
	return pr.Lookup(method, parts)
}

func (pr *prefixRouter) Lookup(method string, path []string) (http.Handler, map[string]string) {
	routes := []string{}
	for p := range pr.staticRoutes {
		routes = append(routes, p)
	}
	for p := range pr.paramRoutes {
		routes = append(routes, ":" + p)
	}
	if len(path) > 0 {
		var trailing []string
		if len(path) > 1 {
			trailing = path[1:]
		} else {
			trailing = []string{}
		}
		sub, ok := pr.staticRoutes[url.PathEscape(path[0])]
		if ok {
			h, params := sub.Lookup(method, trailing)
			if h != nil {
				return h, params
			}
		}
		for param, sub := range pr.paramRoutes {
			h, params := sub.Lookup(method, trailing)
			if h != nil {
				params[param] = path[0]
				return h, params
			}
		}
	}
	h, ok := pr.compiled[method]
	if ok {
		params := map[string]string{}
		if len(path) > 0 {
			params["filepath"] = strings.Join(path, "/")
		}
		return h, params
	}
	sub, ok := pr.paramRoutes[""]
	if ok {
		h, params := sub.Lookup(method, []string{""})
		if h != nil {
			return h, params
		}
	}
	return nil, nil
}

func (pr *prefixRouter) Use(mw Middleware) {
	pr.middlewares = append(pr.middlewares, mw)
}

func (pr *prefixRouter) Prefix(pth string) Router {
	pth = strings.TrimPrefix(path.Clean(pth), "/")
	if pth == "" || pth == "." {
		return pr
	}
	parts := strings.SplitN(pth, "/", 2)
	trailing := ""
	if len(parts) > 1 {
		if parts[1] == "" {
			trailing = ":"
		} else {
			trailing = parts[1]
		}
	}
	me, err := url.PathUnescape(parts[0])
	if err != nil {
		me = parts[0]
	}
	var sub Router
	var ok bool
	if strings.HasPrefix(me, ":") {
		sub, ok = pr.paramRoutes[me[1:]]
		if !ok {
			sub = NewRouter()
			pr.paramRoutes[me[1:]] = sub
		}
	} else {
		sub, ok = pr.staticRoutes[me]
		if !ok {
			sub = NewRouter()
			pr.staticRoutes[me] = sub
		}
	}
	if sub == pr {
		return pr
	}
	return sub.Prefix(trailing)
}

func (pr *prefixRouter) Handle(method, pth string, handler http.Handler) error {
	if pth == "" {
		if _, ok := pr.handlers[method]; ok {
			return errors.New("duplicate route")
		}
		pr.handlers[method] = handler
		return nil
	}
	sub := pr.Prefix(pth)
	err := sub.Handle(method, "", handler)
	if err != nil {
		return errors.Errorf("duplicate route: %s %s", method, pth)
	}
	return nil
}

func (pr *prefixRouter) GET(path string, handler http.Handler) error {
	return pr.Handle(http.MethodGet, path, handler)
}

func (pr *prefixRouter) POST(path string, handler http.Handler) error {
	return pr.Handle(http.MethodPost, path, handler)
}

func (pr *prefixRouter) PUT(path string, handler http.Handler) error {
	return pr.Handle(http.MethodPut, path, handler)
}

func (pr *prefixRouter) PATCH(path string, handler http.Handler) error {
	return pr.Handle(http.MethodPatch, path, handler)
}

func (pr *prefixRouter) DELETE(path string, handler http.Handler) error {
	return pr.Handle(http.MethodDelete, path, handler)
}

func (pr *prefixRouter) OPTIONS(path string, handler http.Handler) error {
	return pr.Handle(http.MethodOptions, path, handler)
}
