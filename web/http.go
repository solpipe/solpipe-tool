package web

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"
)

const HEADER_NAME = "Internal-Route"
const HEADER_JSON_RPC = "jsonrpc"
const HEADER_GRPC = "grpc"

func (e1 external) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	log.Debugf("serving url=%s  or uri path=%s", r.URL.String(), r.URL.Path)
	routeName := r.Header[HEADER_NAME]
	if 0 < len(routeName) {
		switch strings.ToLower(routeName[0]) {
		case HEADER_JSON_RPC:
			log.Debug("going to json rpc")
			if r.Header.Get("Upgrade") == "websocket" {
				e1.ws_proxy(w, r, e1.wsUrl)
			} else {
				e1.proxy_http(w, r, e1.rpcUrl)
			}
		case HEADER_GRPC:
			log.Debug("going to grpc")
			// grpc package name is admin, so we filter by that in the uri path
			if r.Header.Get("Upgrade") == "websocket" {
				e1.ws_proxy(w, r, e1.grpcWebUrl)
			} else {
				e1.proxy_http(w, r, e1.grpcWebUrl)
			}
		default:
			log.Debugf("error with header=%s value=%s", HEADER_NAME, routeName[0])
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}

	// Implement route forwarding
	// by default, send all requests to the React front end
	log.Debugf("url path=%s", r.URL.Path)
	switch r.URL.Path {
	case "/health/startup":
		log.Debug("start up")
		e1.startup(w)
	case "/health/liveness":
		log.Debug("liveness")
		e1.liveness(w)
	case "/agent":
		log.Debug("going to agent")
		if r.Header.Get("Upgrade") == "websocket" {
			log.Debug("serving websocket")
			e1.ws_server_http(w, r)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	default:
		log.Debug("going to default")
		if r.Header.Get("Upgrade") == "websocket" {
			e1.ws_proxy(w, r, e1.frontendUrl)
		} else {
			e1.proxy_http(w, r, e1.frontendUrl)
		}
	}
}

// pass on http requests to a front end that may be running React or Vue.
func (e1 external) proxy_http(w http.ResponseWriter, r *http.Request, remote *url.URL) {

	if remote == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(remote)

	proxy.ServeHTTP(w, r)
}
