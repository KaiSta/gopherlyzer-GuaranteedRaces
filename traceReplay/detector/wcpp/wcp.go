package wcpp

import (
	"fmt"

	"../../util"
	algos "../analysis"
	"../report"
	"../traceReplay"
)

type ListenerAsyncSnd struct{}
type ListenerAsyncRcv struct{}
type ListenerSync struct{}
type ListenerDataAccess struct{}
type ListenerDataAccessDefault struct{}
type ListenerDataAccessDefaultWRD struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}
type ListenerPostProcess struct{}
type ListenerNT struct{}
type ListenerNTWT struct{}

var debug = false
var annotatedTrace = make([]event, 0)

func addToAnnotatedTrace(vc vcepoch, ev *util.Item, ls map[uint32]struct{}) {
	if debug {
		newEvent := event{ct: vc.clone(), ev: ev, ls: make(map[uint32]struct{})}
		for k := range ls {
			newEvent.ls[k] = struct{}{}
		}
		annotatedTrace = append(annotatedTrace, newEvent)
	}
}

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
	signalList = make(map[uint32][]vcepoch)
	variables = make(map[uint32]*variable)
	notifies = make(map[uint32]vcepoch)
	volatiles = make(map[uint32]vcepoch)

	global_acqll = make(map[uint32][]vcepoch)
	global_rell = make(map[uint32][]vcepoch)

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
	algos.RegisterDetector("wcpp", &EventCollector{listeners})

	listeners2 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerDataAccessDefault{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("wcpDefault", &EventCollector{listeners2})

	listeners3 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerDataAccessDefaultWRD{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("wcpDefaultWRD", &EventCollector{listeners3})
}

var threads map[uint32]*thread
var locks map[uint32]*lock
var signalList map[uint32][]vcepoch
var variables map[uint32]*variable
var volatiles map[uint32]vcepoch
var notifies map[uint32]vcepoch

var global_acqll map[uint32][]vcepoch
var global_rell map[uint32][]vcepoch

type thread struct {
	th    vcepoch
	pt    vcepoch //wcp predecessor time
	locks map[uint32]struct{}
	ct    vcepoch
	ht    vcepoch
	wrd   vcepoch
	acqll map[uint32][]vcepoch
	rell  map[uint32][]vcepoch
	id    uint32
	nt    uint32
}

func newT(id uint32) *thread {
	t := &thread{id: id, th: newvc2().set(id, 1), pt: newvc2(),
		locks: make(map[uint32]struct{}), ct: newvc2().set(id, 1),
		ht:    newvc2().set(id, 1),
		acqll: make(map[uint32][]vcepoch), rell: make(map[uint32][]vcepoch), nt: 1, wrd: newvc2()}

	//clone global acqll list
	for k, v := range global_acqll {
		t.acqll[k] = append(v[:0:0], v...)
	}
	//clone global rell list
	for k, v := range global_rell {
		t.rell[k] = append(v[:0:0], v...)
	}
	return t
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
	return &lock{rel: newvc2(), pl: newvc2(), hl: newvc2(),
		acql: make([]lockEvent, 0), rell: make([]lockEvent, 0),
		reads:  make([]uint32, 0),
		writes: make([]uint32, 0), Lrx: make(map[uint32]vcepoch),
		Lwx: make(map[uint32]vcepoch)}
}

type variable struct {
	lwevent   *event
	lastWrite vcepoch
	races     []datarace
	rvc       vcepoch //read wcp time
	wvc       vcepoch //write wcp time
	writes    []*event
	reads     []*event
}

func newV() *variable {
	return &variable{races: make([]datarace, 0), rvc: newvc2(), wvc: newvc2()}
}

type event struct {
	ct vcepoch
	ls map[uint32]struct{}
	ev *util.Item
}

type datarace struct {
	ev1 *event
	ev2 *event
}

func intersect(a, b map[uint32]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; ok {
			return true
		}
	}
	return false
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
	//fmt.Println("LenAcq:", len(acqList), "LenRel:", len(relList))
	for i := 0; i < len(acqList) && i < len(relList) && acqList[i].leq(t1.ct); i++ {
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
	list := global_rell[p.T2]
	list = append(list, t1.ht.clone())
	global_rell[p.T2] = list

	t1.nt++
	t1.ct = t1.pt.clone().set(p.T1, t1.nt)
	t1.ht = t1.ht.add(p.T1, 1)
	addToAnnotatedTrace(t1.ct, p.Ev, t1.locks)
	delete(t1.locks, p.T2)
	threads[p.T1] = t1
	locks[p.T2] = mutex
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

	mutex.writes = make([]uint32, 0)
	mutex.reads = make([]uint32, 0)

	// t1.nt++
	t1.ct = t1.pt.clone().set(p.T1, t1.nt)
	addToAnnotatedTrace(t1.ct, p.Ev, t1.locks)

	for _, t := range threads {
		if t.id != p.T1 {
			list := t.acqll[p.T2]
			list = append(list, t1.ct.clone())
			t.acqll[p.T2] = list
		}
	}
	list := global_acqll[p.T2]
	list = append(list, t1.ct.clone())
	global_acqll[p.T2] = list

	threads[p.T1] = t1
	locks[p.T2] = mutex

}

var countFP = 0

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

	var ct vcepoch
	if p.Write {
		for k := range t1.locks {
			l := locks[k]
			vc1 := l.Lwx[p.T2]
			vc2 := l.Lrx[p.T2]
			t1.pt = t1.pt.ssync(vc1).ssync(vc2)
			l.writes = append(l.writes, p.T2)
		}
		ct = t1.pt.clone().set(t1.id, t1.nt)

		evt := &event{ev: p.Ev, ct: ct.clone(), ls: make(map[uint32]struct{})}
		for k := range t1.locks {
			evt.ls[k] = struct{}{}
		}

		newWrites := make([]*event, 0, len(varstate.writes))
		for i, w := range varstate.writes {
			k := w.ct.get(w.ev.Thread)
			curr := ct.get(w.ev.Thread)
			//if !w.ct.leq(ct) {
			if k > curr {
				newWrites = append(newWrites, varstate.writes[i])

				b := report.ReportRace(
					report.Location{File: w.ev.Ops[0].SourceRef, Line: w.ev.Ops[0].Line, W: true},
					report.Location{File: evt.ev.Ops[0].SourceRef, Line: evt.ev.Ops[0].Line, W: true},
					false, 0)
				if b {
					b = intersect(w.ls, evt.ls)
					if b {
						countFP++
					}
					fmt.Println(">>>", b, countFP)
				}
			}
		}
		newWrites = append(newWrites, evt)
		varstate.writes = newWrites

		for _, r := range varstate.reads {
			k := r.ct.get(r.ev.Thread)
			curr := ct.get(r.ev.Thread)
			//if !r.ct.leq(ct) {
			if k > curr {
				b := report.ReportRace(
					report.Location{File: r.ev.Ops[0].SourceRef, Line: r.ev.Ops[0].Line, W: false},
					report.Location{File: evt.ev.Ops[0].SourceRef, Line: evt.ev.Ops[0].Line, W: true},
					false, 0)
				if b {
					b = intersect(r.ls, evt.ls)
					if b {
						countFP++
					}
					fmt.Println(">>>", b, countFP)
				}
			}
		}
	} else if p.Read {
		for k := range t1.locks {
			l := locks[k]
			vc1 := l.Lwx[p.T2]
			t1.pt = t1.pt.ssync(vc1)
			l.reads = append(l.reads, p.T2)
		}

		ct = t1.pt.clone().set(t1.id, t1.nt)

		evt := &event{ev: p.Ev, ct: ct.clone(), ls: make(map[uint32]struct{})}
		for k := range t1.locks {
			evt.ls[k] = struct{}{}
		}

		for _, w := range varstate.writes {
			k := w.ct.get(w.ev.Thread)
			curr := ct.get(w.ev.Thread)
			//if !w.ct.leq(ct) {
			if k > curr {
				b := report.ReportRace(
					report.Location{File: w.ev.Ops[0].SourceRef, Line: w.ev.Ops[0].Line, W: true},
					report.Location{File: evt.ev.Ops[0].SourceRef, Line: evt.ev.Ops[0].Line, W: true},
					false, 0)
				if b {
					b = intersect(w.ls, evt.ls)
					if b {
						countFP++
					}
					fmt.Println(">>>", b, countFP)
				}
			}
		}

		newReads := make([]*event, 0, len(varstate.reads))
		for _, r := range varstate.reads {
			k := r.ct.get(r.ev.Thread)
			curr := ct.get(r.ev.Thread)
			//if !r.ct.leq(ct) {
			if k > curr {
				newReads = append(newReads, r)
			}
		}
		newReads = append(newReads, evt)
		varstate.reads = newReads
	} else { //volatile synchronize
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

	addToAnnotatedTrace(t1.ct, p.Ev, t1.locks)

	t1.ct = ct //t1.pt.clone().set(p.T1, t1.nt)
	threads[p.T1] = t1
	variables[p.T2] = varstate
}

func (l *ListenerDataAccessDefault) Put(p *util.SyncPair) {
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

	var ct vcepoch
	if p.Write {
		for k := range t1.locks {
			l := locks[k]
			vc1 := l.Lwx[p.T2]
			vc2 := l.Lrx[p.T2]
			t1.pt = t1.pt.ssync(vc1).ssync(vc2)
			l.writes = append(l.writes, p.T2)
		}
		ct = t1.pt.clone().set(t1.id, t1.nt)

		evt := &event{ev: p.Ev, ct: ct, ls: make(map[uint32]struct{})}
		for k := range t1.locks {
			evt.ls[k] = struct{}{}
		}

		newWrites := make([]*event, 0, len(varstate.writes))
		for i, w := range varstate.writes {
			k := w.ct.get(w.ev.Thread)
			curr := ct.get(w.ev.Thread)
			//if !w.ct.leq(ct) {
			if k > curr {
				newWrites = append(newWrites, varstate.writes[i])

				if !intersect(w.ls, evt.ls) {
					report.ReportRace(
						report.Location{File: 1, Line: 1, W: true},
						report.Location{File: evt.ev.Ops[0].SourceRef, Line: evt.ev.Ops[0].Line, W: true},
						false, 0)
				}
				// if b {
				// 	b = intersect(w.ls, evt.ls)
				// 	if b {
				// 		countFP++
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }
			}
		}
		newWrites = append(newWrites, evt)
		varstate.writes = newWrites

		for _, r := range varstate.reads {
			k := r.ct.get(r.ev.Thread)
			curr := ct.get(r.ev.Thread)
			//if !r.ct.leq(ct) {
			if k > curr {
				if !intersect(r.ls, evt.ls) {
					report.ReportRace(
						report.Location{File: 1, Line: 1, W: true},
						report.Location{File: evt.ev.Ops[0].SourceRef, Line: evt.ev.Ops[0].Line, W: true},
						false, 0)
				}
				// if b {
				// 	b = intersect(r.ls, evt.ls)
				// 	if b {
				// 		countFP++
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }
			}
		}
	} else if p.Read {
		for k := range t1.locks {
			l := locks[k]
			vc1 := l.Lwx[p.T2]
			t1.pt = t1.pt.ssync(vc1)
			l.reads = append(l.reads, p.T2)
		}

		ct = t1.pt.clone().set(t1.id, t1.nt)

		evt := &event{ev: p.Ev, ct: ct, ls: make(map[uint32]struct{})}
		for k := range t1.locks {
			evt.ls[k] = struct{}{}
		}

		for _, w := range varstate.writes {
			k := w.ct.get(w.ev.Thread)
			curr := ct.get(w.ev.Thread)
			//if !w.ct.leq(ct) {
			if k > curr {
				if !intersect(w.ls, evt.ls) {
					report.ReportRace(
						report.Location{File: 1, Line: 1, W: true},
						report.Location{File: evt.ev.Ops[0].SourceRef, Line: evt.ev.Ops[0].Line, W: true},
						false, 0)
				}
				// if b {
				// 	b = intersect(w.ls, evt.ls)
				// 	if b {
				// 		countFP++
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }
			}
		}

		newReads := make([]*event, 0, len(varstate.reads))
		for _, r := range varstate.reads {
			k := r.ct.get(r.ev.Thread)
			curr := ct.get(r.ev.Thread)
			//if !r.ct.leq(ct) {
			if k > curr {
				newReads = append(newReads, r)
			}
		}
		newReads = append(newReads, evt)
		varstate.reads = newReads
	} else { //volatile synchronize
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

	t1.ct = ct //t1.pt.clone().set(p.T1, t1.nt)
	addToAnnotatedTrace(t1.ct, p.Ev, t1.locks)
	threads[p.T1] = t1
	variables[p.T2] = varstate
}

func (l *ListenerDataAccessDefaultWRD) Put(p *util.SyncPair) {
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

	var ct vcepoch
	if p.Write {
		for k := range t1.locks {
			l := locks[k]
			vc1 := l.Lwx[p.T2]
			vc2 := l.Lrx[p.T2]
			t1.pt = t1.pt.ssync(vc1).ssync(vc2)
			l.writes = append(l.writes, p.T2)
		}
		ct = t1.pt.clone().set(t1.id, t1.nt)

		evt := &event{ev: p.Ev, ct: ct, ls: make(map[uint32]struct{})}
		for k := range t1.locks {
			evt.ls[k] = struct{}{}
		}

		newWrites := make([]*event, 0, len(varstate.writes))
		for i, w := range varstate.writes {
			k := w.ct.get(w.ev.Thread)

			curr := ct.get(w.ev.Thread)
			if t1.wrd != nil {
				curr = max(curr, t1.wrd.get(w.ev.Thread))
			}
			//if !w.ct.leq(ct) {
			if k > curr {
				newWrites = append(newWrites, varstate.writes[i])
				if !intersect(w.ls, evt.ls) {
					report.ReportRace(
						report.Location{File: 1, Line: 1, W: true},
						report.Location{File: evt.ev.Ops[0].SourceRef, Line: evt.ev.Ops[0].Line, W: true},
						false, 0)
				}
				// if b {
				// 	b = intersect(w.ls, evt.ls)
				// 	if b {
				// 		countFP++
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }
			}
		}
		newWrites = append(newWrites, evt)
		varstate.writes = newWrites
		varstate.lwevent = evt
		varstate.lastWrite = t1.pt.clone().set(t1.id, t1.nt)

		for _, r := range varstate.reads {
			k := r.ct.get(r.ev.Thread)
			//curr := ct.get(r.ev.Thread)
			curr := ct.get(r.ev.Thread)
			if t1.wrd != nil {
				curr = max(curr, t1.wrd.get(r.ev.Thread))
			}
			//if !r.ct.leq(ct) {
			if k > curr {
				if !intersect(r.ls, evt.ls) {
					report.ReportRace(
						report.Location{File: 1, Line: 1, W: true},
						report.Location{File: evt.ev.Ops[0].SourceRef, Line: evt.ev.Ops[0].Line, W: true},
						false, 0)
				}
				// if b {
				// 	b = intersect(r.ls, evt.ls)
				// 	if b {
				// 		countFP++
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }
			}
		}
	} else if p.Read {
		for k := range t1.locks {
			l := locks[k]
			vc1 := l.Lwx[p.T2]
			t1.pt = t1.pt.ssync(vc1)
			l.reads = append(l.reads, p.T2)
		}

		ct = t1.pt.clone().set(t1.id, t1.nt)

		if varstate.lwevent != nil {
			k := varstate.lwevent.ct.get(varstate.lwevent.ev.Thread)
			//curr := ct.get(varstate.lwevent.ev.Thread)
			curr := ct.get(varstate.lwevent.ev.Thread)
			if t1.wrd != nil {
				curr = max(curr, t1.wrd.get(varstate.lwevent.ev.Thread))
			}

			if k > curr {
				if !intersect(varstate.lwevent.ls, t1.locks) {
					report.ReportRace(
						report.Location{File: 1, Line: 1, W: true},
						report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true},
						false, 0)
				}
				// if b {
				// 	b = intersect(varstate.lwevent.ls, t1.locks)
				// 	if b {
				// 		countFP++
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }
			}
			t1.wrd = varstate.lastWrite.clone()
			ct = t1.pt.clone().set(t1.id, t1.nt)
		}

		evt := &event{ev: p.Ev, ct: ct, ls: make(map[uint32]struct{})}
		for k := range t1.locks {
			evt.ls[k] = struct{}{}
		}

		for _, w := range varstate.writes {
			k := w.ct.get(w.ev.Thread)
			//curr := ct.get(w.ev.Thread)
			curr := ct.get(w.ev.Thread)
			if t1.wrd != nil {
				curr = max(curr, t1.wrd.get(w.ev.Thread))
			}
			//if !w.ct.leq(ct) {
			if k > curr {
				if !intersect(w.ls, evt.ls) {
					report.ReportRace(
						report.Location{File: 1, Line: 1, W: true},
						report.Location{File: evt.ev.Ops[0].SourceRef, Line: evt.ev.Ops[0].Line, W: true},
						false, 0)
				}
				// if b {
				// 	b = intersect(w.ls, evt.ls)
				// 	if b {
				// 		countFP++
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }
			}
		}

		newReads := make([]*event, 0, len(varstate.reads))
		for _, r := range varstate.reads {
			k := r.ct.get(r.ev.Thread)
			curr := ct.get(r.ev.Thread)
			//if !r.ct.leq(ct) {
			if k > curr {
				newReads = append(newReads, r)
			}
		}
		newReads = append(newReads, evt)
		varstate.reads = newReads
	} else { //volatile synchronize
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

	addToAnnotatedTrace(t1.ct, p.Ev, t1.locks)

	t1.ct = ct //t1.pt.clone().set(p.T1, t1.nt)
	threads[p.T1] = t1
	variables[p.T2] = varstate
}

//Put for ListenerGoFork handles Forks
func (l *ListenerGoFork) Put(p *util.SyncPair) {
	if !p.IsFork {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}
	ct := t1.pt.clone().set(t1.id, t1.nt)
	signalList[p.T2] = []vcepoch{ct.ssync(t1.ht), t1.wrd}

	addToAnnotatedTrace(t1.ct, p.Ev, t1.locks)

	t1.nt++
	t1.ct = t1.pt.clone().set(p.T1, t1.nt)
	threads[p.T1] = t1
}

//Put for ListenerGoWait handles Waits
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

		t1.ht = t2[0].clone()
		t1.pt = t2[0].clone()
		t1.ct = t1.pt.clone().set(p.T1, t1.nt)
		t1.wrd = t2[1].clone()
		addToAnnotatedTrace(t1.ct, p.Ev, t1.locks)

		// t1.nt++
		t1.ct = t1.pt.clone().set(p.T1, t1.nt)
		threads[p.T1] = t1
	}
}

//Put for ListenerNT handles Notifies
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

	addToAnnotatedTrace(t1.ct, p.Ev, t1.locks)

	t1.nt++
	t1.ct = t1.pt.clone().set(p.T1, t1.nt)
	threads[p.T1] = t1
}

//Put for ListenerNTWT handles NotifyWaits
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

		addToAnnotatedTrace(t1.ct, p.Ev, t1.locks)

		// t1.nt++
		t1.ct = t1.pt.clone().set(p.T1, t1.nt)

		threads[p.T1] = t1
		notifies[p.T2] = vc
	}

}

//Put for ListListenerPostProcess handles postprocessing steps
func (l *ListenerPostProcess) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}
	report.ReportFinish()

	for _, e := range annotatedTrace {
		fmt.Printf("%v:\t%v\t%v\n", e.ev, e.ct, e.ls)
	}

}
