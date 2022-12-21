package scheduler

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
	TRIGGER_CRANK                      EventType = 100 // pipeline: payout.bids
	TRIGGER_CLOSE_BIDS                 EventType = 101 // pipeline: payout.bids
	TRIGGER_CLOSE_PAYOUT               EventType = 102 // pipeline: payout
	TRIGGER_VALIDATOR_SET_PAYOUT       EventType = 103 // validator: payout
	TRIGGER_VALIDATOR_WITHDRAW_RECEIPT EventType = 104 // validator: payout
	TRIGGER_STAKER_ADD                 EventType = 105 // staker: validator, payout
	TRIGGER_STAKER_WITHDRAW            EventType = 106 // staker: validator, payout
	TRIGGER_PERIOD_APPEND              EventType = 107
	TRIGGER_RECEIPT_APPROVE            EventType = 108
)
