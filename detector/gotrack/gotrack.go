package goTrack

import (
	"../../util"
	"../analysis"
	"../report"
	"../traceReplay"
)

type ListenerAsyncSnd struct{}
type ListenerAsyncRcv struct{}
type ListenerChanClose struct{}
type ListenerOpClosedChan struct{}
type ListenerSync struct{}
type ListenerDataAccess struct{}
type ListenerDataAccess2 struct{}
type ListenerSelect struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}

type EventCollector struct{}

var listeners []traceReplay.EventListener

func Init() {
	threads = make(map[uint64]util.VectorClock)
	channels = make(map[uint64]channel)
	variables = make(map[uint64]variable)
	closedChans = make(map[uint64]util.VectorClock)
	signalList = make(map[uint64]util.VectorClock)

	listeners = []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerChanClose{},
		&ListenerOpClosedChan{},
		&ListenerSync{},
		&ListenerDataAccess2{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&traceReplay.Stepper{},
	}
	algos.RegisterDetector("gotrack", &EventCollector{})
}

func (l *EventCollector) Put(m *traceReplay.Machine, p *util.SyncPair) {
	for _, l := range listeners {
		l.Put(m, p)
	}
}

var threads map[uint64]util.VectorClock
var channels map[uint64]channel
var variables map[uint64]variable
var closedChans map[uint64]util.VectorClock
var signalList map[uint64]util.VectorClock

type channel struct {
	buf   []util.VectorClock
	next  uint
	rnext uint
	count int
	lastR util.VectorClock
	lastS util.VectorClock
}

type variable struct {
	rvc       util.VectorClock
	wvc       util.VectorClock
	lastWrite util.VectorClock
	lastOp    *util.Item
}

func newchan(bufsize uint32) channel {
	c := channel{make([]util.VectorClock, bufsize), 0, 0, 0,
		util.NewVC(), util.NewVC()}
	for i := range c.buf {
		c.buf[i] = util.NewVC()
	}

	return c
}
func newvar() variable {
	return variable{util.NewVC(), util.NewVC(), util.NewVC(), nil}
}

func handleLock2(m *traceReplay.Machine, p *util.SyncPair) {
	t1 := m.Threads[p.T1]
	ev := t1.Peek()

	ch, ok := channels[p.T2]
	if !ok {
		bufsize := ev.Ops[0].BufSize
		ch = newchan(bufsize)
	}

	t1VC, ok := threads[p.T1]
	if !ok {
		t1VC = util.NewVC()
	}

	if ev.Ops[0].Mutex&util.LOCK > 0 || ev.Ops[0].Mutex&util.RLOCK > 0 {
		if ch.count > 0 {
			return
		}
		t1VC.Add(t1.ID, 2)
		t1VC.Sync(ch.buf[0])
		t1VC.Sync(ch.buf[1])
		ch.count = 2
	} else if ev.Ops[0].Mutex&util.UNLOCK > 0 || ev.Ops[0].Mutex&util.RUNLOCK > 0 {
		if ch.count != 2 {
			return
		}
		t1VC.Add(t1.ID, 2)
		t1VC.Sync(ch.buf[0])
		t1VC.Sync(ch.buf[1])
		ch.count = 0
	}

	threads[p.T1] = t1VC
	channels[p.T2] = ch
}

func (l *ListenerAsyncSnd) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.AsyncSend {
		return
	}

	t1 := m.Threads[p.T1]
	ev := t1.Peek()

	if ev.Ops[0].Mutex&util.LOCK > 0 || ev.Ops[0].Mutex&util.RLOCK > 0 {
		handleLock2(m, p)
		return
	}

	ch, ok := channels[p.T2]
	if !ok {
		bufsize := ev.Ops[0].BufSize
		ch = newchan(bufsize)
	}

	if ch.count == len(ch.buf) { // no free slot
		return
	}

	t1VC, ok := threads[p.T1]
	if !ok {
		t1VC = util.NewVC()
	}

	t1VC.Add(t1.ID, 2)
	t1VC.Sync(ch.lastS) //updates both
	t1VC.Sync(ch.buf[ch.next])

	ch.count++
	ch.next = (ch.next + 1) % uint(len(ch.buf))

	threads[p.T1] = t1VC
	channels[p.T2] = ch
}

func (l *ListenerAsyncRcv) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.AsyncRcv {
		return
	}

	t1 := m.Threads[p.T1]
	ev := t1.Peek()

	if ev.Ops[0].Mutex&util.UNLOCK > 0 || ev.Ops[0].Mutex&util.RUNLOCK > 0 {
		handleLock2(m, p)
		return
	}

	ch, ok := channels[p.T2]
	if !ok {
		bufsize := ev.Ops[0].BufSize
		ch = newchan(bufsize)
	}

	if ch.count == 0 { //nothing to receive
		return
	}

	t1VC, ok := threads[p.T1]
	if !ok {
		t1VC = util.NewVC()
	}
	t1VC.Add(t1.ID, 2)
	t1VC.Sync(ch.lastR) // updates both
	t1VC.Sync(ch.buf[ch.rnext])

	ch.count--
	ch.rnext = (ch.rnext + 1) % uint(len(ch.buf))

	threads[p.T1] = t1VC
	channels[p.T2] = ch
}

func (l *ListenerChanClose) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.DoClose {
		return
	}
	t1VC, ok := threads[p.T1]
	if !ok {
		t1VC = util.NewVC()
	}

	t1VC.Add(p.T1, 2)

	closedChans[p.T2] = t1VC.Clone()
	threads[p.T1] = t1VC
}

func (l *ListenerOpClosedChan) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.Closed {
		return
	}
	t1VC, ok := threads[p.T1]
	if !ok {
		t1VC = util.NewVC()
	}

	t1VC.Add(p.T1, 2)
	cvc := closedChans[p.T2].Clone()
	t1VC.Sync(cvc)

	threads[p.T1] = t1VC
}

func (l *ListenerSync) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.Sync {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = util.NewVC()
	}
	t2, ok2 := threads[p.T2]
	if !ok2 {
		t2 = util.NewVC()
	}

	//prepare + commit
	t1.Add(p.T1, 2)
	t2.Add(p.T2, 2)
	t1.Sync(t2) //sync updates both

	threads[p.T1] = t1
	threads[p.T2] = t2
}

func (l *ListenerDataAccess2) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.DataAccess {
		return
	}
	thread := m.Threads[p.T1]
	ev := thread.Peek()

	t1VC, ok := threads[p.T1]
	if !ok {
		t1VC = util.NewVC()
	}
	t1VC.Add(thread.ID, 1)

	varstate, ok := variables[ev.Ops[0].Ch]
	if !ok {
		varstate = newvar()
	}

	isAtomic := ev.Ops[0].Kind&util.ATOMICREAD > 0 || ev.Ops[0].Kind&util.ATOMICWRITE > 0

	//BUG: x++ is a read + write, read detects race, replaces lastOp with the
	// read op, next is the write, detects the write-write race
	// but last op is set to the read of the same op, error report is nonsense
	// therefore. Update it similar to threadsanitizer with history for last op.
	if ev.Ops[0].Kind&util.WRITE > 0 {
		varstate.lastWrite = t1VC.Clone()
		if !varstate.wvc.Less(t1VC) {
			if !isAtomic {
				report.Race(&varstate.lastOp.Ops[0], &ev.Ops[0], report.SEVERE)
			}
			varstate.wvc.Set(p.T1, t1VC[p.T1])
		} else {
			varstate.wvc = util.NewVC()
			varstate.wvc.Set(p.T1, t1VC[p.T1])
		}

		if !varstate.rvc.Less(t1VC) {
			if !isAtomic {
				report.Race(&varstate.lastOp.Ops[0], &ev.Ops[0], report.SEVERE)
			}
		}
	} else if ev.Ops[0].Kind&util.READ > 0 {
		if !varstate.wvc.Less(t1VC) {
			if !isAtomic {
				report.Race(&varstate.lastOp.Ops[0], &ev.Ops[0], report.SEVERE)
			}
		}
		if !varstate.rvc.Less(t1VC) {
			varstate.rvc.Set(p.T1, t1VC[p.T1])
		} else {
			varstate.rvc = util.NewVC()
			varstate.rvc.Set(p.T1, t1VC[p.T1])
		}
		t1VC.Sync(varstate.lastWrite.Clone())
	}

	varstate.lastOp = ev

	variables[ev.Ops[0].Ch] = varstate
	threads[p.T1] = t1VC
}

func (l *ListenerGoFork) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.IsFork {
		return
	}
	t1 := m.Threads[p.T1]
	ev := t1.Peek()
	t1VC, ok := threads[p.T1]
	if !ok {
		t1VC = util.NewVC()
	}
	t1VC.Add(t1.ID, 1)

	signalList[ev.Ops[0].Ch] = t1VC.Clone()

	threads[p.T1] = t1VC
}

func (l *ListenerGoWait) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.IsWait {
		return
	}

	t1 := m.Threads[p.T1]
	ev := t1.Peek()

	vc, ok := signalList[ev.Ops[0].Ch]

	if ok {
		t1VC, ok := threads[p.T1]
		if !ok {
			t1VC = util.NewVC()
		}
		t1VC.Sync(vc.Clone())
		t1VC.Add(p.T1, 1)
		threads[p.T1] = t1VC
	}
}
