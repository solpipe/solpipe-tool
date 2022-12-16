package pricing

import (
	ll "github.com/solpipe/solpipe-tool/ds/list"
)

func (pi *periodInfo) start() uint64 {
	return pi.period.Start
}
func (pi *periodInfo) finish() uint64 {
	return pi.period.Start + pi.period.Length - 1
}

func (in *internal) on_period(update periodUpdate) {
	if update.data.Period.IsBlank {
		return
	}
	pi, present := in.pipelineM[update.pipelineId.String()]
	if !present {
		return
	}

	list := pi.periodList

	start := update.data.Period.Start
	finish := update.data.Period.Start + update.data.Period.Length - 1
	info := &periodInfo{
		period:       update.data.Period,
		bs:           update.bs,
		pipelineInfo: pi,
		projectedTps: 0,
	}
	tail := list.TailNode()
	head := list.HeadNode()
	didNotFind := false
	var targetNode *ll.Node[*periodInfo]
	if tail == nil {
		targetNode = list.Append(info)
	} else if tail.Value().finish() < start {
		targetNode = list.Append(info)
	} else if finish < head.Value().start() {
		targetNode = list.Prepend(info)
	} else {
		// need to cycle from head
		didNotFind = true
	foundNode:
		for node := head; node != nil; node = node.Next() {
			if finish < node.Value().start() {
				targetNode = list.Insert(info, node)
				didNotFind = false
				break foundNode
			} else if start == node.Value().start() {
				// duplicate
				//didNotFind = false
				return
			}
		}
		if didNotFind {
			panic("bad algorithm")
		}
	}
	in.periodM[update.payout.Id.String()] = targetNode
}
