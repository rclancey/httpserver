package httpserver

import (
	"context"
	"net/http"

	//"github.com/gorilla/mux"
	"github.com/rclancey/logging"
	"github.com/gofrs/uuid"
)

type reqCtxKey string

func CreateRequestContext(srv *Server, req *http.Request) *http.Request {
	reqId, err := uuid.NewV1()
	if err != nil {
		return req
	}
	log, err := srv.cfg.Logging.ErrorLogger()
	if err != nil {
		return req
	}
	log = log.WithPrefix(reqId.String())
	ctx := req.Context()
	ctx = context.WithValue(ctx, reqCtxKey("reqId"), reqId.String())
	ctx = logging.NewContext(ctx, log)
	return req.Clone(ctx)
}

func ContextRequestId(ctx context.Context) string {
	reqId, ok := ctx.Value(reqCtxKey("reqId")).(string)
	if !ok {
		return ""
	}
	return reqId
}

func ContextRequestVars(ctx context.Context) map[string]string {
	v, ok := ctx.Value(reqCtxKey("vars")).(map[string]string)
	if !ok {
		return map[string]string{}
	}
	return v
}
