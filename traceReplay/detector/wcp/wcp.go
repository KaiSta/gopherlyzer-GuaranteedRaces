package wcp

import (
	"fmt"

	"../../util"
	algos "../analysis"
	"../report"
	"../traceReplay"
)

var debug = false

func addToAnnotatedTrace(vc vcepoch, ev *util.Item) {
	if debug {
		annotatedTrace = append(annotatedTrace, event{vc.clone(), ev})
	}
}

type ListenerAsyncSnd struct{}
type ListenerAsyncRcv struct{}
type ListenerDataAccess struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}
type ListenerPostProcess struct{}
type ListenerNT struct{}
type ListenerNTWT struct{}

type EventCollector struct {
	listeners []traceReplay.EventListener
}

func (l *EventCollector) Put(p *util.SyncPair) {
	for _, l := range l.listeners {
		l.Put(p)
	}
}

func Init() {
	locks = make(map[uint32]*lock)
	threads = make(map[uint32]*thread)
	signalList = make(map[uint32]vcepoch)
	variables = make(map[uint32]*variable)
	notifies = make(map[uint32]vcepoch)
	volatiles = make(map[uint32]vcepoch)

	listeners := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerDataAccess{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("wcp", &EventCollector{listeners})
}

var threads map[uint32]*thread
var locks map[uint32]*lock
var signalList map[uint32]vcepoch
var variables map[uint32]*variable
var volatiles map[uint32]vcepoch
var notifies map[uint32]vcepoch

type thread struct {
	th    vcepoch
	pt    vcepoch //wcp predecessor time
	locks map[uint32]struct{}
	ct    vcepoch
	ht    vcepoch
	acqll map[uint32][]vcepoch
	rell  map[uint32][]vcepoch
	id    uint32
	nt    uint32
}

func newT(id uint32) *thread {
	return &thread{id: id, th: newvc2().set(id, 1), pt: newvc2(), locks: make(map[uint32]struct{}), ct: newvc2().set(id, 1), ht: newvc2().set(id, 1),
		acqll: make(map[uint32][]vcepoch), rell: make(map[uint32][]vcepoch), nt: 1}
}

type lockEvent struct {
	t  uint32
	vc vcepoch
}

type lock struct {
	rel    vcepoch
	pl     vcepoch //wcp predecessor time
	hl     vcepoch
	acql   []lockEvent
	rell   []lockEvent
	reads  []uint32
	writes []uint32
	Lrx    map[uint32]vcepoch
	Lwx    map[uint32]vcepoch
}

func newL() *lock {
	return &lock{rel: newvc2(), pl: newvc2(), acql: make([]lockEvent, 0),
		rell: make([]lockEvent, 0), reads: make([]uint32, 0),
		writes: make([]uint32, 0), Lrx: make(map[uint32]vcepoch),
		Lwx: make(map[uint32]vcepoch), hl: newvc2()}
}

type variable struct {
	lastWrite vcepoch
	races     []datarace
	rvc       vcepoch //read wcp time
	wvc       vcepoch //write wcp time
}

func newV() *variable {
	return &variable{lastWrite: newvc2(), races: make([]datarace, 0), rvc: newvc2(), wvc: newvc2()}
}

type event struct {
	vc vcepoch
	ev *util.Item
}

var annotatedTrace = make([]event, 0)

type datarace struct {
	ev1 *event
	ev2 *event
}

func (l *ListenerAsyncSnd) Put(p *util.SyncPair) {
	if !p.AsyncSend || !p.Lock {
		return
	}

	mutex, ok := locks[p.T2]
	if !ok {
		mutex = newL()
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	t1.ht = t1.ht.ssync(mutex.hl)
	t1.pt = t1.pt.ssync(mutex.pl)
	t1.locks[p.T2] = struct{}{} //add lock to lockset

	mutex.writes = make([]uint32, 0, len(mutex.writes))
	mutex.reads = make([]uint32, 0, len(mutex.reads))

	addToAnnotatedTrace(t1.ct, p.Ev)

	// t1.nt++
	t1.ct = t1.pt.clone().set(p.T1, t1.nt)

	for _, t := range threads {
		if t.id != p.T1 {
			list := t.acqll[p.T2]
			list = append(list, t1.ct.clone())
			t.acqll[p.T2] = list
		}
	}

	threads[p.T1] = t1
	locks[p.T2] = mutex
}

func (l *ListenerAsyncRcv) Put(p *util.SyncPair) {
	if !p.AsyncRcv || !p.Unlock {
		return
	}

	mutex, ok := locks[p.T2]
	if !ok {
		mutex = newL()
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	acqList := t1.acqll[p.T2]
	relList := t1.rell[p.T2]
	lowerBound := 0
	for i := 0; i < len(acqList) && acqList[i].leq(t1.ct); i++ {
		lowerBound++
		t1.pt = t1.pt.ssync(relList[i])
	}
	acqList = acqList[lowerBound:]
	relList = relList[lowerBound:]

	for _, x := range mutex.reads {
		vc := mutex.Lrx[x]
		if vc == nil {
			vc = newvc2()
		}
		ct := t1.pt.clone().set(t1.id, t1.nt)
		vc = vc.ssync(ct)
		vc = vc.ssync(t1.ht)
		mutex.Lrx[x] = vc
	}

	for _, x := range mutex.writes {
		vc := mutex.Lwx[x]
		if vc == nil {
			vc = newvc2()
		}
		ct := t1.pt.clone().set(t1.id, t1.nt)
		vc = vc.ssync(ct)
		vc = vc.ssync(t1.ht)
		mutex.Lwx[x] = vc
	}

	ct := t1.pt.clone().set(t1.id, t1.nt)
	mutex.hl = mutex.hl.ssync(ct)
	mutex.hl = mutex.hl.ssync(t1.ht)
	mutex.pl = mutex.pl.ssync(t1.pt)

	for _, t := range threads {
		if t.id != p.T1 {
			list := t.rell[p.T2]
			list = append(list, t1.ht.clone())
			t.rell[p.T2] = list
		}
	}

	addToAnnotatedTrace(t1.ct, p.Ev)

	t1.nt++
	t1.ct = t1.pt.clone().set(p.T1, t1.nt)
	delete(t1.locks, p.T2)
	threads[p.T1] = t1
	locks[p.T2] = mutex
}

func (l *ListenerDataAccess) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newV()
	}

	if p.Write {
		for k := range t1.locks {
			l := locks[k]
			vc1 := l.Lwx[p.T2]
			vc2 := l.Lrx[p.T2]
			t1.pt = t1.pt.ssync(vc1).ssync(vc2)
			l.writes = append(l.writes, p.T2)
		}
		ct := t1.pt.clone().set(t1.id, t1.nt)
		addToAnnotatedTrace(ct, p.Ev)

		if !varstate.wvc.leq(ct) {
			report.ReportRace(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
		}
		if !varstate.rvc.leq(ct) {
			report.ReportRace(report.Location{File: 1, Line: 1, W: false},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
		}
		varstate.wvc = varstate.wvc.ssync(ct)
	} else if p.Read {
		for k := range t1.locks {
			l := locks[k]
			vc1 := l.Lwx[p.T2]
			t1.pt = t1.pt.ssync(vc1)
			l.reads = append(l.reads, p.T2)
		}

		ct := t1.pt.clone().set(t1.id, t1.nt)
		addToAnnotatedTrace(ct, p.Ev)

		if !varstate.wvc.leq(ct) {
			report.ReportRace(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, false, 0)
		}

		varstate.rvc = varstate.rvc.ssync(ct)

	} else {
		vol, ok := volatiles[p.T2]
		if !ok {
			vol = newvc2()
		}

		t1.ht = t1.ht.ssync(vol)
		t1.pt = t1.pt.ssync(vol)

		ct := t1.pt.clone().set(t1.id, t1.nt)
		vol = ct.ssync(t1.ht)
		volatiles[p.T2] = vol
	}

	//t1.nt++
	t1.ct = t1.pt.clone().set(p.T1, t1.nt)
	threads[p.T1] = t1
	variables[p.T2] = varstate
}

func (l *ListenerGoFork) Put(p *util.SyncPair) {
	if !p.IsFork {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}
	ct := t1.pt.clone().set(t1.id, t1.nt)
	signalList[p.T2] = ct.ssync(t1.ht)

	addToAnnotatedTrace(t1.ct, p.Ev)

	t1.nt++
	t1.ct = t1.pt.clone().set(p.T1, t1.nt)
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
			t1 = newT(p.T1)
		}

		t1.ht = t2.clone()
		t1.pt = t2.clone()
		t1.ct = t1.pt.clone().set(p.T1, t1.nt)
		addToAnnotatedTrace(t1.ct, p.Ev)

		// t1.nt++
		t1.ct = t1.pt.clone().set(p.T1, t1.nt)
		threads[p.T1] = t1
	}
}

func (l *ListenerNT) Put(p *util.SyncPair) {
	if !p.IsNT {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}
	vc, ok := notifies[p.T2]
	if !ok {
		vc = newvc2()
	}

	ct := t1.pt.clone().set(t1.id, t1.nt)
	vc = ct.ssync(t1.ht)

	notifies[p.T2] = vc

	addToAnnotatedTrace(t1.ct, p.Ev)

	t1.nt++
	t1.ct = t1.pt.clone().set(p.T1, t1.nt)
	threads[p.T1] = t1
}

func (l *ListenerNTWT) Put(p *util.SyncPair) {
	if !p.IsNTWT {
		return
	}

	//post wait event, so notify is already synchronized
	if vc, ok := notifies[p.T2]; ok {
		t1, ok := threads[p.T1]
		if !ok {
			t1 = newT(p.T1)
		}

		t1.ht = t1.ht.ssync(vc)
		t1.pt = t1.pt.ssync(vc)

		ct := t1.pt.clone().set(t1.id, t1.nt)
		vc = ct.ssync(t1.ht)

		addToAnnotatedTrace(t1.ct, p.Ev)

		// t1.nt++
		t1.ct = t1.pt.clone().set(p.T1, t1.nt)

		threads[p.T1] = t1
		notifies[p.T2] = vc
	}

}

func (l *ListenerPostProcess) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}

	for _, e := range annotatedTrace {
		fmt.Printf("%v:\t%v\n", e.ev, e.vc)
	}
}
