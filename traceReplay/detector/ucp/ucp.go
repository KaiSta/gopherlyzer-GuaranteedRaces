package ucp

import (
	"../../util"
	"../analysis"
	"../report"
	"../traceReplay"
)

type ListenerAsyncSnd struct{}
type ListenerAsyncRcv struct{}
type ListenerSync struct{}
type ListenerDataAccess struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}
type ListenerPostProcess struct{}
type ListenerPreBranch struct{}
type ListenerPostBranch struct{}

type EventCollector struct {
	listeners []traceReplay.EventListener
}

func (l *EventCollector) Put(p *util.SyncPair) {
	for _, l := range l.listeners {
		l.Put(p)
	}
}

func Init() {
	locks = make(map[uint32]lock)
	threads = make(map[uint32]thread)
	signalList = make(map[uint32]vcepoch)
	variables = make(map[uint32]variable)

	listeners := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccess{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerPostProcess{},
		&ListenerPreBranch{},
		&ListenerPostBranch{},
	}
	algos.RegisterDetector("ucp", &EventCollector{listeners})
}

var threads map[uint32]thread
var locks map[uint32]lock
var signalList map[uint32]vcepoch
var variables map[uint32]variable

type thread struct {
	th         vcepoch
	wrd        vcepoch
	openBranch uint
}
type lock struct {
	rel vcepoch
}
type variable struct {
	lastWrite vcepoch
	races     []datarace
	cw        []*event
	cr        []*event
}

type event struct {
	evtVC vcepoch
	ev    *util.Item
}

type datarace struct {
	ev1 *event
	ev2 *event
}

func (l *ListenerAsyncRcv) Put(p *util.SyncPair) {
	if !p.AsyncRcv || !p.Unlock {
		return
	}

	mutex, ok := locks[p.T2]
	if !ok {
		mutex = lock{rel: newvc2()}
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = thread{openBranch: 0, th: newvc2().set(p.T1, 1), wrd: newvc2()}
	}

	mutex.rel = t1.th.clone()
	t1.th = t1.th.add(p.T1, 1)

	threads[p.T1] = t1
	locks[p.T2] = mutex
}

func (l *ListenerAsyncSnd) Put(p *util.SyncPair) {
	if !p.AsyncSend || !p.Lock {
		return
	}

	mutex, ok := locks[p.T2]
	if !ok {
		mutex = lock{rel: newvc2()}
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = thread{openBranch: 0, th: newvc2().set(p.T1, 1), wrd: newvc2()}
	}

	t1.th = t1.th.ssync(mutex.rel)

	threads[p.T1] = t1
}

func (l *ListenerSync) Put(p *util.SyncPair) {
}

func (l *ListenerDataAccess) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = thread{openBranch: 0, th: newvc2().set(p.T1, 1), wrd: newvc2()}
	}

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = variable{lastWrite: newvc2(), races: make([]datarace, 0), cw: make([]*event, 0), cr: make([]*event, 0)}
	}

	var VC = t1.th.clone()
	if t1.openBranch > 0 {
		VC = t1.th.union(t1.wrd)
	}

	if p.Write {
		newEvent := &event{ev: p.Ev, evtVC: VC.clone()}
		newCw := make([]*event, 0, len(varstate.cw))

		for _, write := range varstate.cw {
			k := write.evtVC.get(write.ev.Thread)
			thi_at_j := VC.get(write.ev.Thread)

			if k > thi_at_j {
				newCw = append(newCw, write)
				report.RaceStatistics2(report.Location{File: write.ev.Ops[0].SourceRef, Line: write.ev.Ops[0].Line, W: true},
					report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
			}
		}

		newCw = append(newCw, newEvent)

		for _, read := range varstate.cr {
			k := read.evtVC.get(read.ev.Thread)
			thi_at_j := VC.get(read.ev.Thread)

			if k > thi_at_j {
				report.RaceStatistics2(report.Location{File: read.ev.Ops[0].SourceRef, Line: read.ev.Ops[0].Line, W: false},
					report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
			}
		}
		varstate.cw = newCw
		varstate.lastWrite = VC.clone()
		t1.th = t1.th.add(p.T1, 1)

	} else if p.Read {
		newEvent := &event{ev: p.Ev, evtVC: VC.clone()}
		newCr := make([]*event, 0, len(varstate.cr))

		for _, write := range varstate.cw {
			k := write.evtVC.get(write.ev.Thread)
			thi_at_j := VC.get(write.ev.Thread)

			if k > thi_at_j {
				report.RaceStatistics2(report.Location{File: write.ev.Ops[0].SourceRef, Line: write.ev.Ops[0].Line, W: true},
					report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, false, 0)
			}
		}

		for _, read := range varstate.cr {
			k := read.evtVC.get(read.ev.Thread)
			thi_at_j := VC.get(read.ev.Thread)

			if k > thi_at_j {
				newCr = append(newCr, read)
			}
		}
		newCr = append(newCr, newEvent)
		varstate.cr = newCr
		t1.wrd = t1.wrd.ssync(varstate.lastWrite)
		t1.th = t1.th.add(p.T1, 1)
	} else {
		panic("Data access but not a write and not a read??")
	}

	threads[p.T1] = t1
	variables[p.T2] = varstate
}

func (l *ListenerGoFork) Put(p *util.SyncPair) {
	if !p.IsFork {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = thread{openBranch: 0, th: newvc2().set(p.T1, 1), wrd: newvc2()}
	}

	signalList[p.T2] = t1.th.clone()

	t1.th = t1.th.add(p.T1, 1)
	threads[p.T1] = t1
}

func (l *ListenerGoWait) Put(p *util.SyncPair) {
	if !p.IsWait {
		return
	}

	t2, ok := signalList[p.T2]

	if ok {
		t1, ok := threads[p.T1]
		if !ok {
			t1 = thread{openBranch: 0, th: newvc2().set(p.T1, 1), wrd: newvc2()}
		}

		t1.th = t1.th.ssync(t2)
		t1.th = t1.th.add(p.T1, 1)
		threads[p.T1] = t1
	}
}

func (l *ListenerPreBranch) Put(p *util.SyncPair) {
	if !p.IsPreBranch {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = thread{openBranch: 0, th: newvc2().set(p.T1, 1), wrd: newvc2()}
	}

	t1.openBranch++
	threads[p.T1] = t1
}
func (l *ListenerPostBranch) Put(p *util.SyncPair) {
	if !p.IsPostBranch {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = thread{openBranch: 0, th: newvc2().set(p.T1, 1), wrd: newvc2()}
	}

	t1.openBranch--
	if t1.openBranch < 0 {
		panic("openBranch negative!")
	}
	threads[p.T1] = t1
}

func (l *ListenerPostProcess) Put(p *util.SyncPair) {
}
