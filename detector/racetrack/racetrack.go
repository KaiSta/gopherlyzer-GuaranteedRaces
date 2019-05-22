package raceTrack

import (
	"../../util"
	"../analysis"
	"../eraser"
	"../report"
	"../traceReplay"
)

type ListenerAsyncSnd struct{}
type ListenerAsyncRcv struct{}
type ListenerSync struct{}
type ListenerDataAccess struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}

type EventCollector struct{}

var listeners []traceReplay.EventListener

func (l *EventCollector) Put(m *traceReplay.Machine, p *util.SyncPair) {
	for _, l := range listeners {
		l.Put(m, p)
	}
}

func Init() {
	threads = make(map[uint64]thread)
	signalList = make(map[uint64]util.VectorClock)
	variables = make(map[uint64]variable)

	listeners = []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccess{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&traceReplay.Stepper{},
	}
	algos.RegisterDetector("racetrack", &EventCollector{})
}

const (
	EXCLUSIVE0    = 1 << iota
	EXCLUSIVE1    = 1 << iota
	SHAREDREAD    = 1 << iota
	SHAREDMODIFY1 = 1 << iota
	EXCLUSIVE2    = 1 << iota
	SHAREDMODIFY2 = 1 << iota
)

var threads map[uint64]thread
var signalList map[uint64]util.VectorClock
var variables map[uint64]variable

type thread struct {
	set map[uint64]struct{}
	vc  util.VectorClock
}

func newThread() thread {
	return thread{make(map[uint64]struct{}), util.NewVC()}
}

type variable struct {
	rvc     util.VectorClock
	wvc     util.VectorClock
	set     map[uint64]struct{}
	lastAcc uint64
	state   uint
	lastOp  *util.Item
}

func newVar(id uint64) variable {
	return variable{util.NewVC(), util.NewVC(), make(map[uint64]struct{}), id, EXCLUSIVE0, nil}
}

func (l *ListenerAsyncSnd) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.AsyncSend {
		return
	}

	t1 := m.Threads[p.T1]
	ev := t1.Peek()

	t1Set, ok := threads[p.T1]
	if !ok {
		t1Set = newThread()
	}
	t1Set.vc.Add(p.T1, 2)

	if ev.Ops[0].Mutex&util.LOCK == 0 {
		return
	}

	t1Set.set[p.T2] = struct{}{}

	threads[p.T1] = t1Set
}

func (l *ListenerAsyncRcv) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.AsyncRcv {
		return
	}

	t1 := m.Threads[p.T1]
	ev := t1.Peek()
	t1Set, ok := threads[p.T1]
	if !ok {
		t1Set = newThread()
	}
	t1Set.vc.Add(p.T1, 2)

	if ev.Ops[0].Mutex&util.LOCK == 0 {
		return
	}

	delete(t1Set.set, p.T2)

	threads[p.T1] = t1Set
}

func (l *ListenerSync) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.Sync {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread()
	}
	t2, ok := threads[p.T2]
	if !ok {
		t2 = newThread()
	}

	//prepare + commit
	t1.vc.Add(p.T1, 2)
	t2.vc.Add(p.T2, 2)
	t1.vc.Sync(t2.vc) //sync updates both

	threads[p.T1] = t1
	threads[p.T2] = t2
}
func (l *ListenerDataAccess) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	thread := m.Threads[p.T1]
	ev := thread.Peek()

	isRead := ev.Ops[0].Kind&util.READ > 0

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar(p.T1)
	}
	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread()
	}
	t1.vc.Add(p.T1, 1)

	switch varstate.state {
	case EXCLUSIVE0:
		if varstate.lastAcc != p.T1 {
			varstate.state = EXCLUSIVE1
			varstate.rvc.Set(p.T1, t1.vc[p.T1])
		}
	case EXCLUSIVE1:
		//threadset merge according to RT
		varstate.rvc = varstate.rvc.Remove(t1.vc)
		varstate.rvc.Set(p.T1, t1.vc[p.T1])
		if len(varstate.rvc) > 1 {
			if isRead {
				varstate.state = SHAREDREAD
			} else {
				varstate.state = SHAREDMODIFY1
			}
		}
	case EXCLUSIVE2:
		//threadset merge according to RT
		varstate.rvc = varstate.rvc.Remove(t1.vc)
		varstate.rvc.Set(p.T1, t1.vc[p.T1])
		if len(varstate.rvc) > 1 {
			varstate.state = SHAREDMODIFY2
		}
	case SHAREDREAD:
		varstate.set = eraser.Intersection(varstate.set, t1.set)
		if !isRead {
			if len(varstate.set) > 0 {
				varstate.state = SHAREDMODIFY1
			} else {
				varstate.state = EXCLUSIVE2
				varstate.rvc = util.NewVC()
				varstate.rvc.Set(p.T1, t1.vc[p.T1])
				varstate.set = t1.set
			}
		}
	case SHAREDMODIFY1:
		varstate.set = eraser.Intersection(varstate.set, t1.set)
		if len(varstate.set) == 0 {
			varstate.state = EXCLUSIVE2
			varstate.rvc = util.NewVC()
			varstate.rvc.Set(p.T1, t1.vc[p.T1])
		}
	case SHAREDMODIFY2:
		//threadset merge according to RT
		varstate.rvc = varstate.rvc.Remove(t1.vc)
		varstate.rvc.Set(p.T1, t1.vc[p.T1])
		varstate.set = eraser.Intersection(varstate.set, t1.set)
		if len(varstate.rvc) == 1 {
			varstate.state = EXCLUSIVE2
		} else if len(varstate.rvc) > 0 && len(varstate.set) == 0 {
			report.Race(&varstate.lastOp.Ops[0], &ev.Ops[0], report.SEVERE)
		}
	}

	varstate.lastAcc = p.T1
	varstate.lastOp = ev

	variables[p.T2] = varstate
	threads[p.T1] = t1
}

func (l *ListenerGoFork) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.IsFork {
		return
	}
	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread()
	}
	t1.vc.Add(p.T1, 1)

	signalList[p.T2] = t1.vc.Clone()

	threads[p.T1] = t1
}
func (l *ListenerGoWait) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.IsWait {
		return
	}

	t1 := m.Threads[p.T1]
	ev := t1.Peek()

	vc, ok := signalList[ev.Ops[0].Ch]

	if ok {
		t1, ok := threads[p.T1]
		if !ok {
			t1 = newThread()
		}
		t1.vc.Sync(vc.Clone())
		t1.vc.Add(p.T1, 1)
		threads[p.T1] = t1
	}
}
