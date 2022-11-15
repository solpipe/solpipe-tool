package web

import (
	"errors"
	"net/http"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
)

var upgrader = websocket.Upgrader{}

func (e1 external) ws_server_http(w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Debug("Error during connection upgradation:", err)
		return
	}

	errorC := make(chan error, 1)
	clientCtx := r.Context()
	doneC := e1.ctx.Done()
	clientDoneC := clientCtx.Done()

	controllerDataC := make(chan cba.Controller)
	go e1.ws_controller(clientCtx, errorC, controllerDataC)

	pipeOut, pipeIn := createPipelinePair()
	go e1.ws_pipeline(clientCtx, errorC, pipeOut)
	pipelineSub := e1.router.ObjectOnPipeline()
	defer pipelineSub.Unsubscribe()

out:
	for {
		select {
		case <-doneC:
			err = errors.New("server done")
			break out
		case <-clientDoneC:
			break out
		case d := <-controllerDataC:
			err = conn.WriteJSON(&d)
			if err != nil {
				break out
			}
		case err = <-pipelineSub.ErrorC:
			break out
		case p := <-pipelineSub.StreamC:
			go e1.ws_on_pipeline(
				errorC,
				clientCtx,
				p,
				pipeOut,
			)
		case x := <-pipeIn.dataC:
			err = conn.WriteJSON(&x)
			if err != nil {
				break out
			}
		case x := <-pipeIn.bidC:
			err = conn.WriteJSON(&x)
			if err != nil {
				break out
			}
		case x := <-pipeIn.periodC:
			err = conn.WriteJSON(&x)
			if err != nil {
				break out
			}
		}
	}
	if err != nil {
		log.Debug(err)
	}
}
