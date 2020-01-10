package checker

import (
	"../util"
)

type EventPair struct {
	T1    uint64
	T1Idx int
	T2    uint64
	T2Idx int
}

func Check(threads map[uint64]util.Thread, pair EventPair) {

}

type AsyncChan struct {
	Bufs []Buffer
}

type Buffer struct {
	t1 *util.Item
	t2 *util.Item
}

func buildGraph(threads map[uint64]util.Thread) map[uint64]util.Thread {
	threadNodes := make(map[uint64]util.Thread)
	asyncChans := make(map[uint64]AsyncChan)

	for k, t := range threads {
		for i, ev := range t.Events {
			if i > 0 {
				t.Events[i-1].Next = append(t.Events[i-1].Next, t.Events[i])
			}

			switch ev.Ops[0].Kind {
			case util.POST | util.SEND:
				if ev.Ops[0].BufSize > 0 {
					handleAsyncSend(ev, threads)
				} else {
					handleSyncSnd(ev, threads)
				}
			case util.POST | util.RCV:
				if ev.Ops[0].BufSize > 0 {
					handleAsyncRcv(ev, threads)
				} else {
					handleSyncRcv(ev, threads)
				}
			case util.POST | util.WRITE:
				handleWrite(ev, threads)
			case util.POST | util.READ:
				handleRead(ev, threads)
			}
		}
	}

}

func handleAsyncSend(item *util.Item, threads map[uint64]util.Thread) {

}
func handleAsyncRcv(item *util.Item, threads map[uint64]util.Thread) {

}
func handleSyncSnd(item *util.Item, threads map[uint64]util.Thread) {

}
func handleSyncRcv(item *util.Item, threads map[uint64]util.Thread) {

}
func handleWrite(item *util.Item, threads map[uint64]util.Thread) {

}
func handleRead(item *util.Item, threads map[uint64]util.Thread) {

}
