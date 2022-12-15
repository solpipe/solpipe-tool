package lite_test

import (
	"context"
	"testing"

	"github.com/solpipe/solpipe-tool/state/pricing/lite"
)

func TestBasic(t *testing.T) {

	fp := "/tmp/test.db"
	ctx, cancel := context.WithCancel(context.Background())
	handle, err := lite.Create(ctx, fp)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	finishC := handle.CloseSignal()
	t.Cleanup(func() {
		cancel()
		<-finishC
	})

	err = handle.Initialize()
	if err != nil {
		t.Fatal(err)
	}

}
