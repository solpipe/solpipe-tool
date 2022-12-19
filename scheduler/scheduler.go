package scheduler

import (
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
}

func Create(eventType EventType, isStateChange bool, slot uint64) Event {
	return Event{
		Slot: slot, Type: eventType, IsStateChange: isStateChange,
	}
}
