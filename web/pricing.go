package web

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

func (e1 external) pricing(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path[len("/pricing"):] {
	case "/slot":
		log.Debug("slot")
		w.WriteHeader(http.StatusNotFound)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}
