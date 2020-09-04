package httpserver

import (
	"encoding/json"
	"fmt"
	"io"
	//"log"
	"net/http"

	"github.com/pkg/errors"
)

type HTTPError interface {
	error
	Cause() error
	Status() string
	StatusCode() int
	Headers() http.Header
	Message() string
	Wrap(error, string) HTTPError
	Wrapf(error, string, ...interface{}) HTTPError
}

type herr struct {
	status int
	name string
	err error
	message string
}

func newHerr(code int, name string) *herr {
	return &herr{
		status: code,
		name: name,
		err: nil,
		message: "",
	}
}

func (e *herr) Error() string {
	return e.err.Error()
}

func (e *herr) Cause() error {
	return e.err
}

func (e *herr) Status() string {
	return e.name
}

func (e *herr) StatusCode() int {
	return e.status
}

func (e *herr) Headers() http.Header {
	return nil
}

func (e *herr) Message() string {
	if e.message == "" {
		return e.name
	}
	return e.message
}

func (e *herr) New(message string) HTTPError {
	return e.Wrap(errors.New(message), "")
}

func (e *herr) Errorf(format string, args ...interface{}) HTTPError {
	return e.Wrap(errors.Errorf(format, args...), "")
}

func (e *herr) Wrap(err error, message string) HTTPError {
	return &herr{
		status: e.status,
		name: e.name,
		err: err,
		message: message,
	}
}

func (e *herr) Wrapf(err error, format string, args ...interface{}) HTTPError {
	return &herr{
		status: e.status,
		name: e.name,
		err: err,
		message: fmt.Sprintf(format, args...),
	}
}

func (e *herr) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"code": e.status,
		"name": e.name,
	}
	if e.status >= 400 {
		m["status"] = "error"
		m["error"] = e.Error()
	} else {
		m["status"] = "OK"
	}
	return json.Marshal(m)
}

func (e *herr) IsA(other error) bool {
	he, isa := other.(*herr)
	if !isa {
		return false
	}
	return e.status == he.status
}

func (e *herr) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			io.WriteString(s, e.message)
			cause := e.Cause()
			if cause != nil {
				f, isa := cause.(fmt.Formatter)
				if isa {
					f.Format(s, verb)
				} else {
					io.WriteString(s, cause.Error())
				}
			}
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, e.message)
	case 'q':
		fmt.Fprintf(s, "%q", e.message)
	}
}

type redirect struct {
	*herr
	location string
}

func newRedirect(code int, name string) *redirect {
	return &redirect{
		newHerr(code, name),
		"/",
	}
}

func (e *redirect) To(location string) *redirect {
	return &redirect{
		e.herr,
		location,
	}
}

func (e *redirect) Message() string {
	return ""
}

func (e *redirect) Headers() http.Header {
	h := http.Header{}
	h.Set("Location", e.location)
	return h
}

type notFound struct {
	*herr
	resource string
}

func newNotFound() *notFound {
	return &notFound{
		newHerr(http.StatusNotFound, "Not Found"),
		"",
	}
}

func (e *notFound) FromRequest(req *http.Request) *notFound {
	return &notFound{
		e.herr,
		req.URL.Path,
	}
}

func (e *notFound) FromResource(rsrc string) *notFound {
	return &notFound{
		e.herr,
		rsrc,
	}
}

func (e *notFound) Message() string {
	if e.resource == "" {
		return e.herr.Message()
	}
	return fmt.Sprintf("Resource %s Not Found", e.resource)
}

type methodNotAllowed struct {
	*herr
	method string
}

func newMethodNotAllowed() *methodNotAllowed {
	return &methodNotAllowed{
		newHerr(http.StatusMethodNotAllowed, "Method Not Allowed"),
		"",
	}
}

func (e *methodNotAllowed) FromRequest(req *http.Request) *methodNotAllowed {
	return &methodNotAllowed{
		e.herr,
		req.Method,
	}
}

func (e *methodNotAllowed) FromMethod(method string) *methodNotAllowed {
	return &methodNotAllowed{
		e.herr,
		method,
	}
}

func (e *methodNotAllowed) Message(w http.ResponseWriter) string {
	if e.method == "" {
		return e.herr.Message()
	}
	return fmt.Sprintf("Method %s Not Allowed", e.method)
}

var OK = newHerr(http.StatusOK, "OK")
var Created = newHerr(http.StatusCreated, "Created")
var NoContent = newHerr(http.StatusNoContent, "No Content")
var Accepted = newHerr(http.StatusAccepted, "Accpeted")
var PartialContent = newHerr(http.StatusPartialContent, "Partial Content")

var MovedPermanently = newRedirect(http.StatusMovedPermanently, "Moved Permanently")
var Found = newRedirect(http.StatusFound, "Found")
var NotModified = newHerr(http.StatusNotModified, "Not Modified")
var TemporaryRedirect = newRedirect(http.StatusTemporaryRedirect, "Temporary Redirect")
var PermanentRedirect = newRedirect(http.StatusPermanentRedirect, "Permanent Redirect")

var BadRequest = newHerr(http.StatusBadRequest, "Bad Request")
var Unauthorized = newHerr(http.StatusUnauthorized, "Login Required")
var Forbidden = newHerr(http.StatusForbidden, "Forbidden")
var NotFound = newNotFound()
var MethodNotAllowed = newMethodNotAllowed()
var NotAcceptable = newHerr(http.StatusNotAcceptable, "Not Acceptable")
var RequestTimeout = newHerr(http.StatusRequestTimeout, "Request Timeout")
var Conflict = newHerr(http.StatusConflict, "Conflict")
var Gone = newHerr(http.StatusGone, "Gone")
var PreconditionFailed = newHerr(http.StatusPreconditionFailed, "Precondition Failed")
var TooManyRequests = newHerr(http.StatusTooManyRequests, "Too Many Requests")

var InternalServerError = newHerr(http.StatusInternalServerError, "Internal Server Error")
var NotImplemented = newHerr(http.StatusNotImplemented, "Not Implemented")
var BadGateway = newHerr(http.StatusBadGateway, "Bad Gateway")
var ServiceUnavailable = newHerr(http.StatusServiceUnavailable, "Service Unavailable")
var GatewayTimeout = newHerr(http.StatusGatewayTimeout, "Gateway Timeout")
