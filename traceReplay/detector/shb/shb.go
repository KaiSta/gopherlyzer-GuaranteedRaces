package shb

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
type ListenerDataAccessNoOp struct{}
type ListenerDataAccessOp struct{}
type ListenerDataAccessNoOp2 struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}
type ListenerNT struct{}
type ListenerNTWT struct{}
type ListenerPostProcess struct{}
type ListenerPostProcess2 struct{}

type EventCollector struct {
	listeners []traceReplay.EventListener
}

var statistics = true
var doPostProcess = true

func Init() {
	threads = make(map[uint32]thread)
	locks = make(map[uint32]vcepoch)
	signalList = make(map[uint32]vcepoch)
	variables = make(map[uint32]variable)
	volatiles = make(map[uint32]vcepoch)
	notifies = make(map[uint32]vcepoch)

	listeners1 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessNoOp{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("shb+", &EventCollector{listeners1})

	listeners2 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessOp{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		//&ListenerPostProcess2{},
	}
	algos.RegisterDetector("shb", &EventCollector{listeners2})

	listeners3 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessNoOp{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess2{},
	}
	algos.RegisterDetector("shb*", &EventCollector{listeners3})
}

var threads map[uint32]thread
var locks map[uint32]vcepoch
var signalList map[uint32]vcepoch
var variables map[uint32]variable
var volatiles map[uint32]vcepoch
var notifies map[uint32]vcepoch

type thread struct {
	vc vcepoch
	ls map[uint32]struct{}
}

func newT(id uint32) thread {
	return thread{vc: newvc2().set(id, 1), ls: make(map[uint32]struct{})}
}

type variable struct {
	lastWrite vcepoch
	lwLS      map[uint32]struct{}
	lwEv      *util.Item
	rvc       vcepoch
	wvc       vcepoch
	lastEv    *util.Item
	history   []variableHistory
	hasRace   bool
	races     []dataRace
}

type dataRace struct {
	pos int
}

type variableHistory struct {
	ls      map[uint32]struct{}
	clock   vcepoch
	ev      *util.Item
	isWrite bool
}

func newVarHistory(ev *util.Item, clock vc2, isWrite bool) variableHistory {
	return variableHistory{isWrite: isWrite, ev: ev, clock: clock, ls: make(map[uint32]struct{})}
}

func newVar() variable {
	return variable{lastWrite: newvc2(), lwEv: nil, rvc: newEpoch(0, 0), wvc: newEpoch(0, 0),
		lastEv: nil, history: make([]variableHistory, 0), hasRace: false, races: make([]dataRace, 0), lwLS: make(map[uint32]struct{})}
}

func (l *EventCollector) Put(p *util.SyncPair) {
	for _, l := range l.listeners {
		l.Put(p)
	}
}

func (l *ListenerAsyncSnd) Put(p *util.SyncPair) {
	if !p.AsyncSend {
		return
	}

	if !p.Lock {
		return
	}

	lock, ok := locks[p.T2]
	if !ok {
		lock = newvc2()
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	t1.vc = t1.vc.ssync(lock)
	t1.ls[p.T2] = struct{}{}

	threads[p.T1] = t1
}
func (l *ListenerAsyncRcv) Put(p *util.SyncPair) {
	if !p.AsyncRcv {
		return
	}

	if !p.Unlock {
		return
	}

	lock, ok := locks[p.T2]
	if !ok {
		lock = newvc2()
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	lock = t1.vc.clone()
	t1.vc = t1.vc.add(p.T1, 1)
	delete(t1.ls, p.T2)

	threads[p.T1] = t1
	locks[p.T2] = lock
}

var raceMap = make(map[report.Location]struct{})
var allRaces uint32
var uniqueRaces uint32

func (l *ListenerDataAccessNoOp) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar()
	}

	if p.Write {
		if !varstate.rvc.leq(t1.vc) {
			varstate.hasRace = true
			varstate.races = append(varstate.races, dataRace{len(varstate.history)})
		}
		if !varstate.wvc.leq(t1.vc) {
			varstate.hasRace = true

			varstate.races = append(varstate.races, dataRace{len(varstate.history)})
		}

		varstate.lastWrite = t1.vc.clone()
		varstate.lwLS = make(map[uint32]struct{})
		for k := range t1.ls {
			varstate.lwLS[k] = struct{}{}
		}
		varstate.lwEv = p.Ev
		varstate.wvc = varstate.wvc.set(p.T1, t1.vc.get(p.T1))

		newHist := variableHistory{clock: t1.vc.clone(), isWrite: true, ev: p.Ev, ls: make(map[uint32]struct{})}
		for k := range t1.ls {
			newHist.ls[k] = struct{}{}
		}
		varstate.history = append(varstate.history, newHist)

		t1.vc = t1.vc.add(p.T1, 1)
	} else if p.Read {
		if !varstate.wvc.leq(t1.vc) {
			varstate.hasRace = true
			varstate.races = append(varstate.races, dataRace{len(varstate.history)})
		}

		if !varstate.lastWrite.leq(t1.vc) && varstate.lwEv != nil {
			varstate.hasRace = true
			b := report.ReportRace(report.Location{File: varstate.lwEv.Ops[0].SourceRef, Line: varstate.lwEv.Ops[0].Line, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, true, 0)

			if b {
				b = intersect(t1.ls, varstate.lwLS)
				if b {
					countFP++
				}
				fmt.Println(">>>", b, countFP)
			}

		}

		t1.vc = t1.vc.ssync(varstate.lastWrite)
		varstate.rvc = varstate.rvc.set(p.T1, t1.vc.get(p.T1))

		newHist := variableHistory{clock: t1.vc.clone(), isWrite: false, ev: p.Ev, ls: make(map[uint32]struct{})}
		for k := range t1.ls {
			newHist.ls[k] = struct{}{}
		}

		varstate.history = append(varstate.history, newHist)

		t1.vc = t1.vc.add(p.T1, 1)
	} else { //volatile synchronize
		vol, ok := volatiles[p.T2]
		if !ok {
			vol = newvc2()
		}
		t1.vc = t1.vc.ssync(vol)
		vol = t1.vc.clone()
		volatiles[p.T2] = vol
		t1.vc = t1.vc.add(p.T1, 1)
	}

	varstate.lastEv = p.Ev
	variables[p.T2] = varstate

	threads[p.T1] = t1
}

func (l *ListenerDataAccessNoOp2) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar()
	}

	if p.Write {
		if !varstate.rvc.less(t1.vc) {
			report.ReportRace(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, true, 0)
		}
		if !varstate.wvc.less(t1.vc) {
			report.ReportRace(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, true, 0)
		}

		varstate.lastWrite = t1.vc.clone()
		varstate.lwEv = p.Ev
		varstate.wvc = varstate.wvc.set(p.T1, t1.vc.get(p.T1))

		t1.vc = t1.vc.add(p.T1, 1)
	} else if p.Read {
		if !varstate.wvc.less(t1.vc) {
			report.ReportRace(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, true, 0)
		}

		t1.vc = t1.vc.ssync(varstate.lastWrite)
		varstate.rvc = varstate.rvc.set(p.T1, t1.vc.get(p.T1))

		t1.vc = t1.vc.add(p.T1, 1)
	} else { //volatile synchronize
		vol, ok := volatiles[p.T2]
		if !ok {
			vol = newvc2()
		}
		t1.vc = t1.vc.ssync(vol)
		vol = t1.vc.clone()
		volatiles[p.T2] = vol
	}

	varstate.lastEv = p.Ev
	variables[p.T2] = varstate

	threads[p.T1] = t1
}

func (l *ListenerDataAccessOp) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar()
	}

	if p.Write {
		if !varstate.rvc.leq(t1.vc) {
			report.ReportRace(report.Location{File: 1, Line: 1, W: false},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
		}

		if varstate.wvc.leq(t1.vc) {
			varstate.wvc = newEpoch(p.T1, t1.vc.get(p.T1))
		} else {
			report.ReportRace(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
		}

		varstate.wvc = varstate.wvc.set(p.T1, t1.vc.get(p.T1))
		varstate.lastWrite = t1.vc.clone()
		varstate.lwEv = p.Ev

		//	varstate.history = append(varstate.history, variableHistory{clock: t1.clone(), isWrite: true, ev: p.Ev})

	} else if p.Read {

		if !varstate.lastWrite.leq(t1.vc) {
			report.ReportRace(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, false, 0)
		}

		t1.vc = t1.vc.ssync(varstate.lastWrite)

		if _, ok := varstate.rvc.(epoch); ok {
			if varstate.rvc.less(t1.vc) {
				varstate.rvc = newEpoch(p.T1, t1.vc.get(p.T1))
			} else {
				varstate.rvc = varstate.rvc.set(p.T1, t1.vc.get(p.T1))
			}
		} else {
			varstate.rvc.set(p.T1, t1.vc.get(p.T1))
		}

		//	varstate.history = append(varstate.history, variableHistory{clock: t1.clone(), isWrite: false, ev: p.Ev})
	} else { //volatile synchronize
		vol, ok := volatiles[p.T2]
		if !ok {
			vol = newvc2()
		}
		t1.vc = t1.vc.ssync(vol)
		vol = t1.vc.clone()
		volatiles[p.T2] = vol
	}

	t1.vc = t1.vc.add(p.T1, 1)
	varstate.lastEv = p.Ev
	variables[p.T2] = varstate
	threads[p.T1] = t1

}

func (l *ListenerSync) Put(p *util.SyncPair) {
	if !p.Sync {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}
	t2, ok := threads[p.T2]
	if !ok {
		t2 = newT(p.T2)
	}

	t1.vc = t1.vc.add(p.T1, 1)
	t2.vc = t2.vc.add(p.T2, 1)

	t1.vc = t1.vc.ssync(t2.vc)

	threads[p.T1] = t1
	threads[p.T2] = t2
}
func (l *ListenerGoFork) Put(p *util.SyncPair) {
	if !p.IsFork { //used for sig - wait too
		return
	}
	t1VC, ok := threads[p.T1]
	if !ok {
		t1VC = newT(p.T1)
	}

	signalList[p.T2] = t1VC.vc.clone()

	t1VC.vc = t1VC.vc.add(p.T1, 1)

	threads[p.T1] = t1VC
}
func (l *ListenerGoWait) Put(p *util.SyncPair) {
	if !p.IsWait {
		return
	}

	vc, ok := signalList[p.T2]

	if ok {
		t1VC, ok := threads[p.T1]
		if !ok {
			t1VC = newT(p.T1)
		}
		t1VC.vc = t1VC.vc.ssync(vc)
		t1VC.vc = t1VC.vc.add(p.T1, 1)
		threads[p.T1] = t1VC
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

	vc = vc.ssync(t1.vc)
	t1.vc = t1.vc.add(p.T1, 1)

	notifies[p.T2] = vc
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

		t1.vc = t1.vc.ssync(vc)
		t1.vc = t1.vc.add(p.T1, 1)
		vc = t1.vc.clone()

		threads[p.T1] = t1
		notifies[p.T2] = vc
	}

}

func intersect(a, b map[uint32]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; ok {
			return true
		}
	}
	return false
}

var countFP = 0

func (l *ListenerPostProcess) Put(p *util.SyncPair) {
	if !p.PostProcess || !doPostProcess {
		return
	}
	fmt.Printf("Races SHB only(no Partner):%v/%v\n", uniqueRaces, allRaces)

	for n, v := range variables {
		// if !v.hasRace {
		// 	continue
		// }
		fmt.Printf("\r%v/%v - %v", n, len(variables), len(v.history))
		for i, x := range v.history {
			//for j, y := range v.history {
			for j := i + 1; j < len(v.history); j++ {
				y := v.history[j]
				//at least one is a write, not the same event, not the same thread
				if !x.isWrite && !y.isWrite || x.ev == y.ev || x.ev.Thread == y.ev.Thread {
					continue
				}

				if !x.clock.less(y.clock) && !y.clock.less(x.clock) {

					//j always greater i
					b := report.ReportRace(report.Location{File: x.ev.Ops[0].SourceRef, Line: x.ev.Ops[0].Line, W: x.isWrite},
						report.Location{File: y.ev.Ops[0].SourceRef, Line: y.ev.Ops[0].Line, W: y.isWrite}, false, 1)

					if b {
						b = intersect(x.ls, y.ls)
						if b {
							countFP++
						}
						fmt.Println(">>>", b, countFP)
					}

					if i > j {
						panic("BS!!!")
					}

				}
			}
		}
	}
}

func (l *ListenerPostProcess2) Put(p *util.SyncPair) {
	if !p.PostProcess || !doPostProcess {
		return
	}
	vv := 1
	for _, v := range variables {
		fmt.Printf("\r%v/%v - %v", vv, len(variables), len(v.history))
		for _, x := range v.races {

			raceEv := v.history[x.pos]
			for j := x.pos; j >= 0; j-- {
				prevEv := v.history[j]
				if !raceEv.isWrite && !prevEv.isWrite || raceEv.ev.Thread == prevEv.ev.Thread {
					continue
				}
				if !raceEv.clock.less(prevEv.clock) && !prevEv.clock.less(raceEv.clock) {
					b := report.ReportRace(
						report.Location{File: prevEv.ev.Ops[0].SourceRef, Line: prevEv.ev.Ops[0].Line, W: prevEv.isWrite},
						report.Location{File: raceEv.ev.Ops[0].SourceRef, Line: raceEv.ev.Ops[0].Line, W: raceEv.isWrite},
						false, 1)
					if b {
						b = intersect(raceEv.ls, prevEv.ls)
						if b {
							countFP++
						}
						fmt.Println(">>>", b, countFP)
					}
				}
			}
		}
		vv++
	}
}
