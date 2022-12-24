package receipt

import (
	"context"

	sch "github.com/solpipe/solpipe-tool/scheduler"
)

// EVENT_PERIOD_FINISH
func loopPeriodClock(
	ctx context.Context,
	ps sch.Schedule,
	errorC chan<- error,
	eventC chan<- sch.Event,
) {
	var err error
	doneC := ctx.Done()
	periodEventSub := ps.OnEvent()
	defer periodEventSub.Unsubscribe()
	var event sch.Event

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-periodEventSub.ErrorC:
			break out
		case event = <-periodEventSub.StreamC:

			switch event.Type {
			case sch.EVENT_PERIOD_FINISH:
				select {
				case <-doneC:
					break out
				case eventC <- event:
				}
				break out
			case sch.EVENT_DELAY_CLOSE_PAYOUT:
				select {
				case <-doneC:
					break out
				case eventC <- event:
				}
				break out
			default:
			}

		}
	}
	if err != nil {
		errorC <- err
	}
}
