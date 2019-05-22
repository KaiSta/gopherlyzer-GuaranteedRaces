package shb

import (
	"fmt"

	"../../util"
	"../analysis"
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
type ListenerPostProcess struct{}
type ListenerPostProcess2 struct{}

type EventCollector struct {
	listeners []traceReplay.EventListener
}

var statistics = true
var doPostProcess = true

func Init() {
	threads = make(map[uint32]vcepoch)
	locks = make(map[uint32]vcepoch)
	signalList = make(map[uint32]vcepoch)
	variables = make(map[uint32]variable)

	listeners1 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessNoOp{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerPostProcess2{},
	}
	algos.RegisterDetector("shb+", &EventCollector{listeners1})

	listeners2 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessOp{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		//&ListenerPostProcess2{},
	}
	algos.RegisterDetector("shb", &EventCollector{listeners2})
}

var threads map[uint32]vcepoch
var locks map[uint32]vcepoch
var signalList map[uint32]vcepoch
var variables map[uint32]variable

type variable struct {
	lastWrite vcepoch
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
	isWrite bool
	ev      *util.Item
	clock   vcepoch
}

func newVarHistory(ev *util.Item, clock vc2, isWrite bool) variableHistory {
	return variableHistory{isWrite, ev, clock}
}

func newVar() variable {
	return variable{newvc2(), nil, newEpoch(0, 0), newEpoch(0, 0),
		nil, make([]variableHistory, 0), false, make([]dataRace, 0)}
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
		t1 = newvc2().set(p.T1, 1)
	}

	t1 = t1.ssync(lock)

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
		t1 = newvc2().set(p.T1, 1)
	}

	lock = t1.clone()
	t1 = t1.add(p.T1, 1)

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
		t1 = newvc2().set(p.T1, 1)
	}

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar()
	}

	if p.Write {
		if !varstate.rvc.leq(t1) {
			varstate.hasRace = true
			varstate.races = append(varstate.races, dataRace{len(varstate.history)})
		}
		if !varstate.wvc.leq(t1) {
			varstate.hasRace = true

			varstate.races = append(varstate.races, dataRace{len(varstate.history)})
		}

		varstate.lastWrite = t1.clone()
		varstate.lwEv = p.Ev
		varstate.wvc = varstate.wvc.set(p.T1, t1.get(p.T1))

		varstate.history = append(varstate.history, variableHistory{clock: t1.clone(), isWrite: true, ev: p.Ev})

		t1 = t1.add(p.T1, 1)
	} else if p.Read {
		if !varstate.wvc.leq(t1) {
			varstate.hasRace = true
			varstate.races = append(varstate.races, dataRace{len(varstate.history)})
		}

		if !varstate.lastWrite.leq(t1) && varstate.lwEv != nil {
			varstate.hasRace = true
			report.RaceStatistics2(report.Location{File: varstate.lwEv.Ops[0].SourceRef, Line: varstate.lwEv.Ops[0].Line, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, true, 0)

		}

		t1 = t1.ssync(varstate.lastWrite)
		varstate.rvc = varstate.rvc.set(p.T1, t1.get(p.T1))

		varstate.history = append(varstate.history, variableHistory{clock: t1.clone(), isWrite: false, ev: p.Ev})

		t1 = t1.add(p.T1, 1)
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
		t1 = newvc2().set(p.T1, 1)
	}

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar()
	}

	if p.Write {
		if !varstate.rvc.less(t1) {
			report.RaceStatistics2(report.Location{File: 1, Line: 1, W: false},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, true, 0)
		}
		if !varstate.wvc.less(t1) {
			report.RaceStatistics2(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, true, 0)
		}

		varstate.lastWrite = t1.clone()
		varstate.lwEv = p.Ev
		varstate.wvc = varstate.wvc.set(p.T1, t1.get(p.T1))

		t1 = t1.add(p.T1, 1)
	} else if p.Read {
		if !varstate.wvc.less(t1) {
			report.RaceStatistics2(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, true, 0)
		}

		t1 = t1.ssync(varstate.lastWrite)
		varstate.rvc = varstate.rvc.set(p.T1, t1.get(p.T1))

		t1 = t1.add(p.T1, 1)
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
		t1 = newvc2().set(p.T1, 1)
	}

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar()
	}

	if p.Write {
		if !varstate.rvc.leq(t1) {
			report.RaceStatistics2(report.Location{File: 1, Line: 1, W: false},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
		}

		if varstate.wvc.leq(t1) {
			varstate.wvc = newEpoch(p.T1, t1.get(p.T1))
		} else {
			report.RaceStatistics2(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
		}

		varstate.wvc = varstate.wvc.set(p.T1, t1.get(p.T1))
		varstate.lastWrite = t1.clone()
		varstate.lwEv = p.Ev

		//	varstate.history = append(varstate.history, variableHistory{clock: t1.clone(), isWrite: true, ev: p.Ev})

	} else if p.Read {

		if !varstate.lastWrite.leq(t1) {
			report.RaceStatistics2(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, false, 0)
		}

		t1 = t1.ssync(varstate.lastWrite)

		if _, ok := varstate.rvc.(epoch); ok {
			if varstate.rvc.less(t1) {
				varstate.rvc = newEpoch(p.T1, t1.get(p.T1))
			} else {
				varstate.rvc = varstate.rvc.set(p.T1, t1.get(p.T1))
			}
		} else {
			varstate.rvc.set(p.T1, t1.get(p.T1))
		}

		//	varstate.history = append(varstate.history, variableHistory{clock: t1.clone(), isWrite: false, ev: p.Ev})
	}
	t1 = t1.add(p.T1, 1)
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
		t1 = newvc2().set(p.T1, 1)
	}
	t2, ok := threads[p.T2]
	if !ok {
		t2 = newvc2().set(p.T2, 1)
	}

	t1 = t1.add(p.T1, 1)
	t2 = t2.add(p.T2, 1)

	t1 = t1.ssync(t2)

	threads[p.T1] = t1
	threads[p.T2] = t2
}
func (l *ListenerGoFork) Put(p *util.SyncPair) {
	if !p.IsFork { //used for sig - wait too
		return
	}
	t1VC, ok := threads[p.T1]
	if !ok {
		t1VC = newvc2().set(p.T1, 1)
	}

	signalList[p.T2] = t1VC.clone()

	t1VC = t1VC.add(p.T1, 1)

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
			t1VC = newvc2().set(p.T1, 1)
		}
		t1VC = t1VC.ssync(vc)
		t1VC = t1VC.add(p.T1, 1)
		threads[p.T1] = t1VC
	}
}

func (l *ListenerPostProcess) Put(p *util.SyncPair) {
	if !p.PostProcess || !doPostProcess {
		return
	}
	fmt.Printf("Races SHB only(no Partner):%v/%v\n", uniqueRaces, allRaces)

	for n, v := range variables {
		if !v.hasRace {
			continue
		}
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
					report.RaceStatistics2(report.Location{File: x.ev.Ops[0].SourceRef, Line: x.ev.Ops[0].Line, W: x.isWrite},
						report.Location{File: y.ev.Ops[0].SourceRef, Line: y.ev.Ops[0].Line, W: y.isWrite}, false, 1)

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
					report.RaceStatistics2(
						report.Location{File: prevEv.ev.Ops[0].SourceRef, Line: prevEv.ev.Ops[0].Line, W: prevEv.isWrite},
						report.Location{File: raceEv.ev.Ops[0].SourceRef, Line: raceEv.ev.Ops[0].Line, W: raceEv.isWrite},
						false, 1)
				}
			}
		}
		vv++
	}
}
