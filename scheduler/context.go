package scheduler

import "context"

func MergeCtx(a context.Context, b context.Context) context.Context {
	ctxC, cancel := context.WithCancel(a)
	go loopCtxClose(a, b, cancel)
	return ctxC
}

func loopCtxClose(
	a context.Context,
	b context.Context,
	cancel context.CancelFunc,
) {
	defer cancel()
	select {
	case <-a.Done():
	case <-b.Done():
	}
}
