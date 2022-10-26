package sub

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
	RespC  chan<- Subscription[T]
	filter func(T) bool
}

func SubscriptionRequest[T any](reqC chan<- ResponseChannel[T], filterCallback func(T) bool) Subscription[T] {
	respC := make(chan Subscription[T], 10)
	reqC <- ResponseChannel[T]{
		RespC:  respC,
		filter: filterCallback,
	}
	return <-respC
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
	for _, v := range sh.subs {
		if v.filter(value) {
			v.streamC <- value
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

func (sh *SubHome[T]) Receive(resp ResponseChannel[T]) {
	id := sh.id
	sh.id++
	streamC := make(chan T, 10)
	errorC := make(chan error, 1)
	sh.subs[id] = &innerSubscription[T]{
		id: id, streamC: streamC, errorC: errorC, filter: resp.filter,
	}
	resp.RespC <- Subscription[T]{id: id, StreamC: streamC, ErrorC: errorC, deleteC: sh.DeleteC}
}
