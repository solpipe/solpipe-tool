package lite_test

import (
	"context"
	"os"
	"testing"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	ts "github.com/solpipe/solpipe-tool/ds/ts"
	"github.com/solpipe/solpipe-tool/ds/ts/lite"
	ntk "github.com/solpipe/solpipe-tool/state/network"
)

func TestBasic(t *testing.T) {

	fp := "file:solpipe?mode=memory&cache=shared"
	os.Remove(fp)
	ctx, cancel := context.WithCancel(context.Background())
	handle, err := lite.Create(ctx, fp, true)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	finishC := handle.CloseSignal()
	t.Cleanup(func() {
		cancel()
		<-finishC
	})
	is := ts.InitialState{
		Start:  1,
		Finish: 10 ^ 9,
	}
	err = handle.Initialize(is)
	if err != nil {
		t.Fatal(err)
	}
	handle, err = lite.Create(ctx, fp, false)
	if err != nil {
		t.Fatal(err)
	}

	count := 50
	var idList []sgo.PublicKey
	{

		var key sgo.PrivateKey
		idList = make([]sgo.PublicKey, count)
		for i := 0; i < len(idList); i++ {
			key, err = sgo.NewRandomPrivateKey()
			if err != nil {
				t.Fatal(err)
			}
			idList[i] = key.PublicKey()
		}
	}

	err = handle.PipelineAdd(idList)
	if err != nil {
		t.Fatal(err)
	}

	{
		list, err := handle.PipelineList()
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != count {
			t.Fatal("bad nubmer of pipelines")
		}
	}

	start := time.Now()
	{
		n := 10 ^ 9
		list := make([]ts.NetworkPoint, n)
		for i := 1; i < len(list); i++ {
			list[i] = ts.NetworkPoint{
				Slot: uint64(i),
				Status: ntk.NetworkStatus{
					WindowSize:                       1,
					AverageTransactionsPerBlock:      1.234,
					AverageTransactionsPerSecond:     0.432,
					AverageTransactionsSizePerSecond: 341.39,
				},
			}
		}
		err = handle.NetworkAppend(list)
		if err != nil {
			t.Fatal(err)
		}

	}
	finish := time.Now()
	if 10 < finish.Unix()-start.Unix() {
		t.Fatal("too slow")
	}
}
