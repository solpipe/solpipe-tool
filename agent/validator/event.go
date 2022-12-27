package validator

import (
	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
)

func (in *internal) on_event(event sch.Event) error {
	log.Debugf("received event: %s", event.String())
	return nil
}
