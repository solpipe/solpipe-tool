package scheduler

import "fmt"

const thresh int = 1000

const (
	EVENT_PERIOD_PRE_START             EventType = 0
	EVENT_PERIOD_START                 EventType = 1
	EVENT_PERIOD_FINISH                EventType = 2
	EVENT_DELAY_CLOSE_PAYOUT           EventType = 3 // 100 slots after EVENT_PERIOD_FINISH
	EVENT_BID_CLOSED                   EventType = 4
	EVENT_BID_FINAL                    EventType = 5
	EVENT_VALIDATOR_IS_ADDING          EventType = 6
	EVENT_VALIDATOR_HAVE_WITHDRAWN     EventType = 7
	EVENT_STAKER_IS_ADDING             EventType = 8
	EVENT_STAKER_HAVE_WITHDRAWN        EventType = 9
	EVENT_STAKER_HAVE_WITHDRAWN_EMPTY  EventType = 10
	EVENT_TYPE_READY_APPEND            EventType = 11
	EVENT_RECEIPT_NEW_COUNT            EventType = 12
	EVENT_RECEIPT_APPROVED             EventType = 13
	TRIGGER_CRANK                      EventType = thresh + 1 // pipeline: payout.bids
	TRIGGER_CLOSE_BIDS                 EventType = thresh + 2 // pipeline: payout.bids
	TRIGGER_CLOSE_PAYOUT               EventType = thresh + 3 // pipeline: payout
	TRIGGER_VALIDATOR_SET_PAYOUT       EventType = thresh + 4 // validator: payout
	TRIGGER_VALIDATOR_WITHDRAW_RECEIPT EventType = thresh + 5 // validator: payout
	TRIGGER_STAKER_ADD                 EventType = thresh + 6 // staker: validator, payout
	TRIGGER_STAKER_WITHDRAW            EventType = thresh + 7 // staker: validator, payout
	TRIGGER_PERIOD_APPEND              EventType = thresh + 8
	TRIGGER_RECEIPT_APPROVE            EventType = thresh + 9
)

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

func (e Event) IsTrigger() bool {
	return thresh <= e.Type
}
