package pipeline_test

import (
	"context"
	"testing"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	sle "github.com/solpipe/solpipe-tool/test/single"
)

func TestSlot(t *testing.T) {
	var err error
	err = godotenv.Load("../../.env")
	if err != nil {
		t.Fatal(err)
	}
	log.SetLevel(log.DebugLevel)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Cleanup(func() {
		time.Sleep(3 * time.Second)
	})
	sbox, err := sle.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	relayConfig := sbox.PipelineConfig()
	err = relayConfig.Check()
	if err != nil {
		t.Fatal(err)
	}
	router, err := relayConfig.Router(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sub := router.Controller.SlotHome().OnSlot()
	defer sub.Unsubscribe()
	doneC := ctx.Done()
	finishC := time.After(2 * time.Minute)
	var slot uint64
out:
	for {
		select {
		case <-doneC:
			break out
		case <-finishC:
			break out
		case err = <-sub.ErrorC:
			break out
		case slot = <-sub.StreamC:
			log.Infof("slot=%d", slot)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
}
