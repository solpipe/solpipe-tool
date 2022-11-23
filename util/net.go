package util

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

type acceptResponseC = chan<- net.Conn

// this is for the client to connect to the server
type PipeConnect struct {
	ctx      context.Context
	cancel   context.CancelFunc
	requestC chan<- acceptResponseC
}

func (le *PipeConnect) Dial(ctx context.Context) (net.Conn, error) {
	err := le.ctx.Err()
	if err != nil {
		return nil, err
	}
	doneC := le.ctx.Done()
	doneC2 := ctx.Done()
	acceptC := make(chan net.Conn)
	select {
	case <-doneC:
		return nil, errors.New("canceled")
	case <-doneC2:
		return nil, errors.New("canceled")
	case le.requestC <- acceptC:
	}

	var ans net.Conn
	select {
	case <-doneC:
		return nil, errors.New("canceled")
	case <-doneC2:
		return nil, errors.New("canceled")
	case ans = <-acceptC:
	}
	return ans, nil
}

// this is for the server (net.Listener)
type myListener struct {
	ctx      context.Context
	cancel   context.CancelFunc
	requestC <-chan acceptResponseC
	t        time.Time
}

func (ml *myListener) Accept() (net.Conn, error) {
	err := ml.ctx.Err()
	if err != nil {
		return nil, err
	}
	var respC acceptResponseC
	doneC := ml.ctx.Done()
	select {
	case <-doneC:
		err = errors.New("canceled")
	case respC = <-ml.requestC:
	}
	if err != nil {
		return nil, err
	}
	conn1, conn2 := net.Pipe()

	select {
	case <-doneC:
		err = errors.New("canceled")
	case respC <- conn2:
	}
	if err != nil {
		return nil, err
	}
	return conn1, nil
}

func (ml *myListener) Close() error {
	ml.cancel()
	return nil
}

type fakeAddr struct {
	netType string
	addr    string
}

func (fk *fakeAddr) Network() string {
	return fk.netType
}

func (fk *fakeAddr) String() string {
	return fmt.Sprintf("%s://%s", fk.Network(), fk.addr)
}

func (ml *myListener) Addr() net.Addr {
	return &fakeAddr{netType: "pipe", addr: fmt.Sprintf("%d", ml.t.Unix())}
}

// create a pipe net.Listener
func PipeListener(ctx context.Context) (net.Listener, *PipeConnect) {
	requestC := make(chan acceptResponseC)
	ctxC, cancel := context.WithCancel(ctx)
	e1 := &myListener{
		ctx:      ctxC,
		cancel:   cancel,
		requestC: requestC,
		t:        time.Now(),
	}
	li := &PipeConnect{
		ctx:      ctxC,
		cancel:   cancel,
		requestC: requestC,
	}

	return e1, li
}
