package web

import (
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
)

func getUpgrader() websocket.Upgrader {
	switch os.Getenv("PROD_ENV") {
	case "DEV":
		return websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
	default:
		return websocket.Upgrader{}
	}
}

var upgrader = getUpgrader()

type Data struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type SlotTick struct {
	Time time.Time `json:"time"`
	Slot uint64    `json:"slot"`
}

const (
	TYPE_SLOT       string = "slot"
	TYPE_CONTROLLER string = "controller"
	TYPE_PIPELINE   string = "pipeline"
	TYPE_PERIOD     string = "period"
	TYPE_BID        string = "bid"
	TYPE_VALIDATOR  string = "validator"
)

func writeConn(conn *websocket.Conn, typeName string, data interface{}) error {
	return conn.WriteJSON(&Data{
		Type: typeName,
		Data: data,
	})
}

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

	slotSub := e1.router.Controller.SlotHome().OnSlot()
	defer slotSub.Unsubscribe()

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
		case slot := <-slotSub.StreamC:
			log.Debugf("slot=%d", slot)
			err = writeConn(conn, TYPE_SLOT, &SlotTick{
				Time: time.Now(),
				Slot: slot,
			})
			if err != nil {
				break out
			}
		case d := <-controllerDataC:
			err = writeConn(conn, TYPE_CONTROLLER, &d)
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
			err = writeConn(conn, TYPE_PIPELINE, &x)
			if err != nil {
				break out
			}
		case x := <-pipeIn.bidC:
			err = writeConn(conn, TYPE_BID, &x)
			if err != nil {
				break out
			}
		case x := <-pipeIn.periodC:
			err = writeConn(conn, TYPE_PERIOD, &x)
			if err != nil {
				break out
			}
		}
	}
	if err != nil {
		log.Debug(err)
	}
}
