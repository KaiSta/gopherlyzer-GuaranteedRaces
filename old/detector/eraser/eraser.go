package eraser

import (
	"../../util"
	"../analysis"
	"../report"
	"../traceReplay"
)

type ListenerAsyncSnd struct{}
type ListenerAsyncRcv struct{}
type ListenerDataAccess struct{}
type ListenerSelect struct{}

type EventCollector struct{}

var listeners []traceReplay.EventListener

func Init() {
	threads = make(map[uint32]thread)
	variables = make(map[uint32]variable)

	listeners = []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerDataAccess{},
	}
	algos.RegisterDetector("eraser", &EventCollector{})
}

func (l *EventCollector) Put(p *util.SyncPair) {
	for _, l := range listeners {
		l.Put(p)
	}
}

var threads map[uint32]thread
var variables map[uint32]variable

type thread struct {
	set map[uint32]struct{}
}
type variable struct {
	set     map[uint32]struct{}
	state   uint
	lastAcc uint32
	lastOp  *util.Item
}

func newThread() thread {
	return thread{make(map[uint32]struct{})}
}
func newVar(id uint32) variable {
	return variable{make(map[uint32]struct{}), util.EXCLUSIVE, id, nil}
}

func (l *ListenerAsyncSnd) Put(p *util.SyncPair) {
	if !p.AsyncSend {
		return
	}

	ev := p.Ev

	if !p.Lock {
		return //channel op, stepper does the necessary steps
	}

	t1Set, ok := threads[p.T1]
	if !ok {
		t1Set = newThread()
	}
	t1Set.set[ev.Ops[0].Ch] = struct{}{}
	threads[p.T1] = t1Set
}

func (l *ListenerAsyncRcv) Put(p *util.SyncPair) {
	if !p.AsyncRcv {
		return
	}

	//check if its a mutex.unlock or not
	ev := p.Ev

	if ev.Ops[0].Mutex&util.UNLOCK == 0 {
		return
	}

	t1Set, ok := threads[p.T1]
	if !ok {
		t1Set = newThread()
	}
	delete(t1Set.set, p.T2)
	threads[p.T1] = t1Set
}

func (l *ListenerDataAccess) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	ev := p.Ev

	isAtomic := ev.Ops[0].Kind&util.ATOMICREAD > 0 || ev.Ops[0].Kind&util.ATOMICWRITE > 0

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar(p.T1)
	}

	isRead := ev.Ops[0].Kind&util.READ > 0

	t1Set, ok := threads[p.T1]
	if !ok {
		t1Set = newThread()
	}

	switch varstate.state {
	case util.EXCLUSIVE:
		if p.T1 != varstate.lastAcc {
			if isRead {
				varstate.state = util.READSHARED
			} else {
				for k, v := range t1Set.set {
					varstate.set[k] = v
				}
				varstate.state = util.SHARED
			}
		}
	case util.READSHARED:
		varstate.set = Intersection(varstate.set, t1Set.set)
		if !isRead {
			varstate.state = util.SHARED
		}
	case util.SHARED:
		varstate.set = Intersection(varstate.set, t1Set.set)
	}

	if varstate.state == util.SHARED && len(varstate.set) == 0 {
		if !isAtomic {
			report.Race3(varstate.lastOp, ev, report.SEVERE)
		}
	}

	varstate.lastAcc = p.T1
	varstate.lastOp = ev

	variables[p.T2] = varstate
	threads[p.T1] = t1Set
}

func Intersection(m1, m2 map[uint32]struct{}) map[uint32]struct{} {
	m3 := make(map[uint32]struct{})

	for k1 := range m1 {
		_, ok := m2[k1]
		if ok {
			m3[k1] = struct{}{}
		}
	}
	return m3
}
