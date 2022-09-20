package httpserver

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

type ProxyURL string

type Redirect string

type StaticFile string

type WebSocket interface {
	Open(*websocket.Conn) error
	ReadPump()
	WritePump()
	Close()
}

type ETaggable interface {
	ETag() string
}

type Datable interface {
	LastModified() time.Time
}

func SetDefaultContentType(w http.ResponseWriter, ct string) {
	h := w.Header()
	if h.Get("Content-Type") == "" {
		h.Set("Content-Type", ct)
	}
}

func GenEtag(f io.Reader) string {
	h := sha1.New()
	io.Copy(h, f)
	sum := h.Sum([]byte{})
	return fmt.Sprintf(`"%s"`, hex.EncodeToString(sum))
}

type HandlerFunc func(w http.ResponseWriter, req *http.Request) (interface{}, error)

func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	obj, err := h(w, req)
	if err != nil {
		sendError(w, req, err)
		return
	}
	if obj != nil {
		switch tobj := obj.(type) {
		case ProxyURL:
			Proxy(w, req, string(tobj))
		case Redirect:
			http.Redirect(w, req, string(tobj), http.StatusFound)
		case StaticFile:
			f, err := os.Open(string(tobj))
			if err == nil {
				w.Header().Set("Etag", GenEtag(f))
				f.Close()
			}
			http.ServeFile(w, req, string(tobj))
		case io.ReadSeeker:
			w.Header().Set("Etag", GenEtag(tobj))
			tobj.Seek(0, io.SeekStart)
			http.ServeContent(w, req, req.URL.Path, time.Now(), tobj)
			closer, isa := tobj.(io.Closer)
			if isa {
				defer closer.Close()
			}
		case []byte:
			w.Header().Set("Etag", GenEtag(bytes.NewReader(tobj)))
			http.ServeContent(w, req, req.URL.Path, time.Now(), bytes.NewReader(tobj))
		case WebSocket:
			log.Println("handling websocket")
			conn, err := upgrader.Upgrade(w, req, nil)
			if err != nil {
				tobj.Close()
				sendError(w, req, err)
				return
			}
			err = tobj.Open(conn)
			if err != nil {
				log.Println("error opening websocket service:", err)
				tobj.Close()
				sendError(w, req, err)
				return
			}
			go tobj.WritePump()
			go tobj.ReadPump()
		case HTTPError:
			data, err := json.Marshal(tobj)
			if err != nil {
				sendError(w, req, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(tobj.StatusCode())
			w.Write(data)
		case *ObjectStream:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			tobj.Stream(w)
		default:
			var modTime time.Time
			var etag string
			dobj, isa := obj.(Datable)
			if isa {
				modTime = dobj.LastModified()
			}
			eobj, isa := obj.(ETaggable)
			if isa {
				etag = eobj.ETag()
			}
			err := CheckPreconditions(req, modTime, etag)
			if err != nil {
				sendError(w, req, err)
				return
			}
			SendJSON(w, obj)
		}
	}
}
