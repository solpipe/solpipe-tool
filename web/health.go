package web

import (
	"context"
	"errors"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type healthInternal struct {
	ctx        context.Context
	hasStarted bool
	isHealthy  bool
}

func loopHealth(
	ctx context.Context,
	healthC <-chan func(*healthInternal),
) {

	doneC := ctx.Done()
	in := new(healthInternal)

out:
	for {
		select {
		case <-doneC:
			break out
		case req := <-healthC:
			req(in)
		}
	}
}

func (e1 external) health(status bool) {
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
	case e1.healthC <- func(hi *healthInternal) {
		hi.isHealthy = status
	}:
	}
}

// mark that the server has successfully started
func (e1 external) has_started() {
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
	case e1.healthC <- func(hi *healthInternal) {
		log.Debug("setting has started=true")
		hi.hasStarted = true
		hi.isHealthy = true
	}:
	}
}

func (e1 external) startup(w http.ResponseWriter) {
	doneC := e1.ctx.Done()

	errorC := make(chan error, 1)
	select {
	case <-doneC:
		errorC <- errors.New("canceled")
	case e1.healthC <- func(hi *healthInternal) {
		if hi.hasStarted {
			log.Debug("marking has started")
			errorC <- nil
		} else {
			log.Debug("marking has NOT started")
			errorC <- errors.New("not started")
		}
	}:
	}
	var err error
	select {
	case err = <-errorC:
	case <-doneC:
		err = errors.New("canceled")
	}
	if err == nil {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (e1 external) liveness(w http.ResponseWriter) {
	doneC := e1.ctx.Done()

	errorC := make(chan error, 1)
	select {
	case <-doneC:
		errorC <- errors.New("canceled")
	case e1.healthC <- func(hi *healthInternal) {
		if hi.isHealthy {
			log.Debug("marking healthy")
			errorC <- nil
		} else {
			log.Debug("marking NOT healthy")
			errorC <- errors.New("not started")
		}
	}:
	}
	var err error
	select {
	case err = <-errorC:
	case <-doneC:
		err = errors.New("canceled")
	}
	if err == nil {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
