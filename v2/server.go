package httpserver

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	//"github.com/gorilla/mux"
	//"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
)

type Middleware func(http.Handler) http.Handler

type Server struct {
	cfg *ServerConfig
	router Router
	docroot http.Handler
	middlewares []Middleware
	servers []*http.Server
}

func NewServer(cfg *ServerConfig) (*Server, error) {
	err := checkRunning(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "server already running")
	}
	router := NewRouter()
	srv := &Server{
		cfg: cfg,
		router: router,
		docroot: nil,
		middlewares: []Middleware{},
		servers: nil,
	}
	srv.docroot = http.FileServer(http.Dir(srv.cfg.DocumentRoot))
	if srv.cfg.DefaultProxy != "" {
		err := srv.SetDefaultProxy(srv.cfg.DefaultProxy)
		if err != nil {
			return nil, err
		}
	}
	srv.Use(srv.AccessLoggerMiddleware())
	srv.Use(srv.ContextMiddleware())
	srv.Use(CompressMiddleware)

	return srv, nil
}

func (srv *Server) SetDefaultHandler(h http.Handler) {
	srv.docroot = h
}

func (srv *Server) SetDefaultProxy(u string) error {
	base, err := url.Parse(u)
	if err != nil {
		return err
	}
	h := func(w http.ResponseWriter, r *http.Request) {
		u := base.ResolveReference(r.URL)
		Proxy(w, r, u.String())
	}
	srv.SetDefaultHandler(http.HandlerFunc(h))
	return nil
}

func (srv *Server) Use(mw Middleware) {
	srv.router.Use(mw)
	srv.middlewares = append(srv.middlewares, mw)
}

func (srv *Server) Prefix(path string) Router {
	return srv.router.Prefix(path)
}

func (srv *Server) Handle(method, path string, handler http.Handler) {
	srv.router.Handle(method, path, handler)
}

func (srv *Server) GET(path string, handler http.Handler) {
	srv.router.GET(path, handler)
}

func (srv *Server) POST(path string, handler http.Handler) {
	srv.router.POST(path, handler)
}

func (srv *Server) PUT(path string, handler http.Handler) {
	srv.router.PUT(path, handler)
}

func (srv *Server) PATCH(path string, handler http.Handler) {
	srv.router.PATCH(path, handler)
}

func (srv *Server) DELETE(path string, handler http.Handler) {
	srv.router.DELETE(path, handler)
}

func (srv *Server) OPTIONS(path string, handler http.Handler) {
	srv.router.OPTIONS(path, handler)
}

func (srv *Server) ContextMiddleware() Middleware {
	mwf := func(handler http.Handler) http.Handler {
		f := func(w http.ResponseWriter, r *http.Request) {
			r = CreateRequestContext(srv, r)
			handler.ServeHTTP(w, r)
		}
		return http.HandlerFunc(f)
	}
	return Middleware(mwf)
}

func (srv *Server) AccessLoggerMiddleware() Middleware {
	accessLog, err := srv.cfg.Logging.AccessLogger()
	if err != nil {
		errlog, _ := srv.cfg.Logging.ErrorLogger()
		if errlog != nil {
			errlog.Errorln("error setting up access logger middleware:", err)
		}
		mwf := func(handler http.Handler) http.Handler {
			return handler
		}
		return Middleware(mwf)
	}
	mwf := func(handler http.Handler) http.Handler {
		f := func(w http.ResponseWriter, r *http.Request) {
			rl := NewResponseLogger(w, r)
			handler.ServeHTTP(w, r)
			rl.WriteLog(accessLog)
		}
		return http.HandlerFunc(f)
	}
	return Middleware(mwf)
}

func (srv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var parts []string
	if r.URL.RawPath != "" && r.URL.RawPath != r.URL.Path {
		parts = strings.Split(strings.TrimPrefix(path.Clean(r.URL.RawPath), "/"), "/")
		for i, part := range parts {
			xpart, err := url.PathUnescape(part)
			if err == nil {
				parts[i] = xpart
			}
		}
	} else {
		parts = strings.Split(strings.TrimPrefix(path.Clean(r.URL.Path), "/"), "/")
	}
	handler, params := srv.router.Lookup(r.Method, parts)
	if handler != nil {
		ctx := context.WithValue(r.Context(), reqCtxKey("vars"), params)
		r = r.Clone(ctx)
		handler.ServeHTTP(w, r)
	} else if r.Method == http.MethodGet {
		srv.docroot.ServeHTTP(w, r)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func (srv *Server) ListenAndServe() error {
	if srv.servers != nil {
		return errors.New("server already running")
	}
	servers := make([]*http.Server, 2)
	srv.servers = servers
	err := ValidateRouter(srv.router)
	if err != nil {
		return err
	}
	srv.router.Compile([]Middleware{})
	h := srv.docroot
	for i := len(srv.middlewares) - 1; i >= 0; i-- {
		h = srv.middlewares[i](h)
	}
	srv.docroot = h
	l, err := srv.cfg.Logging.ErrorLogger()
	if err != nil {
		return errors.Wrap(err, "can't get access logger")
	}
	err = checkRunning(srv.cfg)
	if err != nil {
		return errors.Wrap(err, "server already running")
	}
	err = writePidfile(srv.cfg)
	if err != nil {
		return errors.Wrap(err, "can't write pid file")
	}
	defer removePidfile(srv.cfg)
	wg := &sync.WaitGroup{}
	errch := make(chan error, 10)
	if srv.cfg.Bind.SSL.Enabled() {
		cfg := srv.cfg.Bind.SSL
		addr := fmt.Sprintf(":%d", cfg.Port)
		server := &http.Server{
			Addr: addr,
			Handler: srv,
		}
		servers[0] = server
		wg.Add(1)
		go func() {
			if l == nil {
				log.Println("listening for https on", server.Addr)
			} else {
				l.Infoln("listening for https on", server.Addr)
			}
			err := server.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile)
			servers[0] = nil
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				errch <- err
			}
			wg.Done()
		}()
	}
	if srv.cfg.Bind.Port != 0 {
		addr := fmt.Sprintf(":%d", srv.cfg.Bind.Port)
		server := &http.Server{
			Addr: addr,
			Handler: srv,
		}
		servers[1] = server
		wg.Add(1)
		go func() {
			if l == nil {
				log.Println("listening for http on", addr)
			} else {
				l.Infoln("listening for http on", addr)
			}
			err := server.ListenAndServe()
			servers[1] = nil
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				errch <- err
			}
			wg.Done()
		}()
	}
	wg.Wait()
	close(errch)
	for {
		err, ok := <-errch
		if !ok {
			break
		}
		if l == nil {
			log.Println(err)
		} else {
			l.Error(err)
		}
	}
	return nil
}

func (srv *Server) RegisterWebSocketHub(hub Hub) {
	servers := srv.servers
	if servers == nil {
		return
	}
	for _, server := range srv.servers {
		if server == nil {
			continue
		}
		server.RegisterOnShutdown(func() {
			hub.Stop()
		})
	}
}

func (srv *Server) Shutdown() error {
	servers := srv.servers
	if servers == nil {
		return nil
	}
	var hadErr error
	for i, server := range srv.servers {
		if server == nil {
			continue
		}
		err := server.Shutdown(context.Background())
		if err == nil {
			srv.servers[i] = nil
		} else {
			hadErr = err
		}
	}
	return hadErr
}
