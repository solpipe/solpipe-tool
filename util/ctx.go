package util

import "context"

func MergeCtx(a context.Context, b context.Context) context.Context {
	if a == nil || b == nil {
		panic("a or b is nil")
	}
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
