package sub

import (
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
)

type Subscription[T any] struct {
	id      int
	deleteC chan<- int
	StreamC <-chan T
	ErrorC  <-chan error
}

func (s Subscription[T]) Unsubscribe() {
	s.deleteC <- s.id
}

type innerSubscription[T any] struct {
	id      int
	streamC chan<- T
	errorC  chan<- error
	filter  func(T) bool
}
type ResponseChannel[T any] struct {
	RespC               chan<- Subscription[T]
	filter              func(T) bool
	requestedBufferSize uint16
}

const DEFAULT_STREAM_BUFFER_SIZE uint16 = 10

func SubscriptionRequest[T any](reqC chan<- ResponseChannel[T], filterCallback func(T) bool) Subscription[T] {
	return SubscriptionRequestWithBufferSize(reqC, DEFAULT_STREAM_BUFFER_SIZE, filterCallback)
}

func SubscriptionRequestWithBufferSize[T any](reqC chan<- ResponseChannel[T], bufSize uint16, filterCallback func(T) bool) (sub Subscription[T]) {

	respC := make(chan Subscription[T], 1)
	select {
	case reqC <- ResponseChannel[T]{
		RespC:               respC,
		filter:              filterCallback,
		requestedBufferSize: bufSize,
	}:
	default:
		streamC := make(chan T)
		deleteC := make(chan int, 1)
		errorC := make(chan error, 1)
		errorC <- errors.New("request queue full")
		return Subscription[T]{
			id:      -1,
			ErrorC:  errorC,
			StreamC: streamC,
			deleteC: deleteC,
		}
	}
	sub = <-respC
	return
}

type SubHome[T any] struct {
	id      int
	subs    map[int]*innerSubscription[T]
	DeleteC chan int
	ReqC    chan ResponseChannel[T]
}

func CreateSubHome[T any]() *SubHome[T] {
	reqC := make(chan ResponseChannel[T], 10)
	return &SubHome[T]{
		id: 0, subs: make(map[int]*innerSubscription[T]), DeleteC: make(chan int, 10), ReqC: reqC,
	}
}

func (sh *SubHome[T]) SubscriberCount() int {
	return len(sh.subs)
}

func (sh *SubHome[T]) Broadcast(value T) {
	for id, v := range sh.subs {
		if v.filter(value) {
			select {
			case v.streamC <- value:
			default:
				err := fmt.Errorf("queue is full (sub type=%d)", id)
				log.Debug(err)
				// errorC guaranteed to have 1 empty space
				v.errorC <- err
				delete(sh.subs, id)
			}
		}
	}
}

func (sh *SubHome[T]) Delete(id int) {
	p, present := sh.subs[id]
	if present {
		p.errorC <- nil
		delete(sh.subs, id)
	}
}

// close all subscriptions
func (sh *SubHome[T]) Close() {
	for _, v := range sh.subs {
		v.errorC <- nil
	}
	sh.subs = make(map[int]*innerSubscription[T])
}

func (sh *SubHome[T]) Receive(resp ResponseChannel[T]) chan<- T {
	id := sh.id
	sh.id++
	streamC := make(chan T, resp.requestedBufferSize)
	errorC := make(chan error, 1)
	sh.subs[id] = &innerSubscription[T]{
		id: id, streamC: streamC, errorC: errorC, filter: resp.filter,
	}
	resp.RespC <- Subscription[T]{id: id, StreamC: streamC, ErrorC: errorC, deleteC: sh.DeleteC}
	return streamC
}
