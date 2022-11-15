package web

import (
	"net/http"
	"net/http/httputil"

	log "github.com/sirupsen/logrus"
)

func (e1 external) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Header.Get("Upgrade") == "websocket" {
		log.Debug("serving websocket")
		e1.ws_server_http(w, r)
		return
	}
	log.Debugf("serving url=%s", r.URL.String())
	// Implement route forwarding
	switch r.URL.String() {
	case "/health/startup":
		log.Debug("start up")
		e1.startup(w)
	case "/health/liveness":
		log.Debug("liveness")
		e1.liveness(w)
	default:
		e1.proxy_front(w, r)
	}
}

// pass on http requests to a front end that may be running React or Vue.
func (e1 external) proxy_front(w http.ResponseWriter, r *http.Request) {

	if e1.proxyUrl == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(e1.proxyUrl)

	proxy.ServeHTTP(w, r)
}
