package scheduler

import (
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/util"
)

type Schedule interface {
	util.Base
	OnEvent() dssub.Subscription[Event]
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
