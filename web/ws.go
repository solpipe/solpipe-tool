package web

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
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
	TYPE_SLOT            string = "slot"
	TYPE_CONTROLLER      string = "controller"
	TYPE_PIPELINE        string = "pipeline"
	TYPE_PERIOD          string = "period"
	TYPE_BID             string = "bid"
	TYPE_BID_STATUS      string = "bid_status"
	TYPE_VALIDATOR       string = "validator"
	TYPE_VALIDATOR_STAKE string = "validator_stake"
	TYPE_PAYOUT          string = "payout"
	TYPE_RECEIPT         string = "receipt"
)

func writeConn(conn *websocket.Conn, typeName string, data interface{}) error {
	return conn.WriteJSON(&Data{
		Type: typeName,
		Data: data,
	})
}

func (e1 external) ws_proxy(w http.ResponseWriter, r *http.Request, remote *url.URL) {
	clientCtx := r.Context()
	clientDoneC := clientCtx.Done()
	doneC := e1.ctx.Done()

	var wsRemoteStr string
	remoteStr := remote.String()
	if strings.HasPrefix(remoteStr, "http") {
		wsRemoteStr = "ws" + remoteStr[len("http"):] + r.URL.Path
	} else if strings.HasPrefix(remoteStr, "https") {
		wsRemoteStr = "wss" + remoteStr[len("https"):] + r.URL.Path
	} else {
		wsRemoteStr = remoteStr + r.URL.Path
	}

	if strings.Contains(wsRemoteStr, "3001") && !strings.Contains(wsRemoteStr, "jsonrpc") {
		wsRemoteStr += "ws"
	}
	log.Debugf("websocket proxy to remote=%s with local=%s", wsRemoteStr, r.URL.String())
	connLocal, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Debug("Error during connection upgradation:", err)
		return
	}
	defer connLocal.Close()

	connRemote, _, err := websocket.DefaultDialer.DialContext(e1.ctx, wsRemoteStr, nil)
	if err != nil {
		log.Debugf("websocket(%s) Error: %s", wsRemoteStr, err.Error())
		return
	}
	defer connRemote.Close()

	// local to remote
	errorC := make(chan error, 2)
	go loopMessagePass(connLocal, connRemote, errorC)
	go loopMessagePass(connRemote, connLocal, errorC)

out:

	for {
		select {
		case err = <-errorC:
			log.Debugf("exiting error: %s", err.Error())
			break out
		case <-doneC:
			log.Debug("exiting debug")
			break out
		case <-clientDoneC:
			log.Debug("client is done")
			break out
		}
	}

	if err != nil {
		log.Debug(err)
	}

}

func loopMessagePass(a *websocket.Conn, b *websocket.Conn, errorC chan<- error) {
	var err error
	var m []byte
	var msgType int
	//log.Debugf("websocket a to b")
out:
	for {
		msgType, m, err = a.ReadMessage()
		if err == io.EOF {
			log.Debug("client - 1")
			err = nil
			break out
		} else if err != nil {
			log.Debug("client - 2")
			break out
		}
		//log.Debugf("sending message=%s", string(m))
		err = b.WriteMessage(msgType, m)
		if err == io.EOF {
			log.Debug("client - 3")
			err = nil
			break out
		} else if err != nil {
			log.Debug("client - 4")
			break out
		}
	}
	if err != nil {
		errorC <- err
	}
}

// subscribe to pipeline, validator changes
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

	valOut, valIn := createValidatorPair()
	go e1.ws_validator(clientCtx, errorC, valOut)
	valSub := e1.router.ObjectOnValidator(func(vwd rtr.ValidatorWithData) bool { return true })
	defer valSub.Unsubscribe()

	pOut, pIn := e1.createPayoutPair(clientCtx)
	go e1.ws_payout(clientCtx, errorC, pOut)
	payoutSub := e1.router.ObjectOnPayout()
	defer payoutSub.Unsubscribe()

out:
	for {
		select {
		case <-doneC:
			err = errors.New("server done")
			break out
		case <-clientDoneC:
			break out
		case err = <-slotSub.ErrorC:
			slotSub = e1.router.Controller.SlotHome().OnSlot()
		case slot := <-slotSub.StreamC:
			//log.Debugf("slot=%d", slot)
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
		case x := <-pIn.bidC:
			err = writeConn(conn, TYPE_BID_STATUS, &x)
			if err != nil {
				break out
			}
		case err = <-valSub.ErrorC:
			break out
		case x := <-valSub.StreamC:
			// covers new validators
			go e1.ws_on_validator(
				errorC,
				clientCtx,
				x.V,
				x.Data,
				valOut,
			)
		case info := <-valIn.stakeC:
			err = writeConn(conn, TYPE_VALIDATOR_STAKE, &info)
			if err != nil {
				break out
			}
		case d := <-valIn.dataC:
			err = writeConn(conn, TYPE_VALIDATOR, &d)
			if err != nil {
				break out
			}
		case err = <-payoutSub.ErrorC:
			break out
		case p := <-payoutSub.StreamC:
			// covers new payouts
			data, err := p.Data()
			if err != nil {
				break out
			}
			go e1.ws_on_payout(
				errorC,
				clientCtx,
				pipe.PayoutWithData{
					Id:     p.Id,
					Payout: p,
					Data:   data,
				},
				pOut,
			)
		case d := <-pIn.dataC:
			err = writeConn(conn, TYPE_PAYOUT, &d)
			if err != nil {
				break out
			}
		case d := <-pIn.receiptC:
			err = writeConn(conn, TYPE_RECEIPT, &d)
			if err != nil {
				break out
			}
		}
	}
	if err != nil {
		log.Debug(err)
	}
}
