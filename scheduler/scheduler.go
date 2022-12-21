package scheduler

import (
	"fmt"

	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/util"
)

type Schedule interface {
	util.Base
	OnEvent() dssub.Subscription[Event]
}

type EventType = int

type Event struct {
	Slot          uint64
	Type          EventType
	IsStateChange bool
	Payload       interface{}
}

func (e Event) String() string {
	return fmt.Sprintf("type=%d; slot=%d; is_state_change=%t", e.Type, e.Slot, e.IsStateChange)
}

func Create(eventType EventType, isStateChange bool, slot uint64) Event {
	return Event{
		Slot: slot, Type: eventType, IsStateChange: isStateChange,
	}
}

func CreateWithPayload(eventType EventType, isStateChange bool, slot uint64, payload interface{}) Event {
	return Event{
		Slot: slot, Type: eventType, IsStateChange: isStateChange, Payload: payload,
	}
}
