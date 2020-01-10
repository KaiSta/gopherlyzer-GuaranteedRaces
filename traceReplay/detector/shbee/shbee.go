package shbee

import (
	"fmt"
	"sort"

	"github.com/xojoc/bitset"

	"../../util"
	algos "../analysis"
	"../report"
	"../traceReplay"
)

var debug = true
var statistics = true

type ListenerAsyncSnd struct{}
type ListenerAsyncRcv struct{}
type ListenerSync struct{}
type ListenerDataAccess struct{}
type ListenerDataAccessSound struct{}
type ListenerDataAccessUnsoundOp struct{}
type ListenerDataAccessReadReadDirectOp struct{}
type ListenerDataAccessLastTry struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}
type ListenerNT struct{}
type ListenerNTWT struct{}
type ListenerPostProcess struct{}
type ListenerPostProcess2 struct{}
type ListenerPostProcess3 struct{}

type EventCollector struct {
	listeners  []traceReplay.EventListener
	windowMode bool
	count      uint
}

var limit uint = 10000

func (l *EventCollector) Put(p *util.SyncPair) {
	if l.windowMode {
		l.count++
		if l.count%limit == 0 {
			//	nvariables := make(map[uint32]variable)
			for _, v := range variables {
				v.detectGraphRaces()
			}

			variables = make(map[uint32]variable)
		}
	}
	for _, l := range l.listeners {
		l.Put(p)
	}
}

func Init() {

	locks = make(map[uint32]vcepoch)
	threads = make(map[uint32]thread)
	signalList = make(map[uint32]vcepoch)
	variables = make(map[uint32]variable)
	volatiles = make(map[uint32]vcepoch)
	notifies = make(map[uint32]vcepoch)

	listeners := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessReadReadDirectOp{},
		//&ListenerDataAccessLastTry{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}

	algos.RegisterDetector("shbee", &EventCollector{listeners, false, 0})
	//algos.RegisterDetector("shbeeW", &EventCollector{listeners, true, 0})

	listeners2 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessLastTry{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess2{},
	}
	algos.RegisterDetector("shbe", &EventCollector{listeners2, false, 0})

	// listeners3 := []traceReplay.EventListener{
	// 	&ListenerAsyncSnd{},
	// 	&ListenerAsyncRcv{},
	// 	&ListenerSync{},
	// 	&ListenerDataAccessLastTry{},
	// 	&ListenerGoFork{},
	// 	&ListenerGoWait{},
	// 	&ListenerNT{},
	// 	&ListenerNTWT{},
	// 	&ListenerPostProcess3{},
	// }
	//algos.RegisterDetector("shbeTEST", &EventCollector{listeners3, false, 0})
}

type thread struct {
	vc vcepoch
	ls map[uint32]struct{}
}

func newT(id uint32) thread {
	return thread{vc: newvc2().set(id, 1), ls: make(map[uint32]struct{})}
}

var threads map[uint32]thread
var locks map[uint32]vcepoch
var signalList map[uint32]vcepoch
var variables map[uint32]variable
var volatiles map[uint32]vcepoch
var notifies map[uint32]vcepoch

type pair struct {
	*dot
	a bool
}

type read struct {
	File uint32
	Line uint32
	T    uint32
}

type variable struct {
	races     []datarace
	history   []variableHistory
	frontier  []*dot
	graph     myGraph
	lastWrite vcepoch
	lwDot     *dot
	current   int

	//frontier  map[uint32]*pair
	//graph          map[int][]*dot
	//isWrite        *bitset.BitSet
	//raceFilter     map[int]*bitset.BitSet
	//readReadFilter map[read]map[read]struct{}
	//cache map[int][]*dot
}

type dataRace struct {
	raceAcc int
	prevAcc int
}

type variableHistory struct {
	ls        map[uint32]struct{}
	sourceRef uint32
	t         uint32
	c         uint32
	line      uint16
	isWrite   bool
}

type myGraph struct {
	ds [][]*dot
}

func (g *myGraph) add(dID int, dots []*dot) {
	if (dID - 1) >= len(g.ds) {
		g.ds = append(g.ds, dots)
	} else {
		g.ds[dID-1] = dots
	}
}

func (g *myGraph) get(dID int) []*dot {
	return g.ds[dID-1]
}

// func (v *variable) isNewReadRead(T1, T2 uint32, dot1, dot2 *dot) bool {
// 	r1 := read{T: T1, File: dot1.ev.Ops[0].SourceRef, Line: dot1.ev.Ops[0].Line}
// 	r2 := read{T: T2, File: dot2.ev.Ops[0].SourceRef, Line: dot2.ev.Ops[0].Line}
// 	set1 := v.readReadFilter[r1]
// 	if set1 == nil {
// 		set1 = make(map[read]struct{})
// 	}
// 	set2 := v.readReadFilter[r2]
// 	if set2 == nil {
// 		set2 = make(map[read]struct{})
// 	}

// 	_, ok1 := set1[r2]
// 	_, ok2 := set2[r1]

// 	if ok1 || ok2 {
// 		return false
// 	}

// 	set1[r2] = struct{}{}
// 	set2[r1] = struct{}{}
// 	v.readReadFilter[r1] = set1
// 	v.readReadFilter[r2] = set2
// 	return true
// }

// func (v *variable) addRace(raceAcc, prevAcc *dot) {
// 	return
// 	set1 := v.raceFilter[raceAcc.int]
// 	if set1 == nil {
// 		set1 = &bitset.BitSet{}
// 	}
// 	set1.Set(prevAcc.int)
// 	v.raceFilter[raceAcc.int] = set1

// 	set2 := v.raceFilter[prevAcc.int]
// 	if set2 == nil {
// 		set2 = &bitset.BitSet{}
// 	}
// 	set2.Set(raceAcc.int)
// 	v.raceFilter[prevAcc.int] = set2
// }

// func (v *variable) isNewRace(raceAcc, prevAcc *dot) bool {
// 	return true
// 	set1 := v.raceFilter[raceAcc.int]
// 	if set1 == nil {
// 		set1 = &bitset.BitSet{}
// 	}
// 	set2 := v.raceFilter[prevAcc.int]
// 	if set2 == nil {
// 		set2 = &bitset.BitSet{}
// 	}

// 	return !set1.Get(prevAcc.int) || !set2.Get(raceAcc.int)
// }

type dot struct {
	int
	ls        map[uint32]struct{}
	v         vcepoch
	sourceRef uint32
	pos       int
	line      uint16
	t         uint16
	write     bool
}

type datarace struct {
	d1 *dot
	d2 *dot
}

func newVar() variable {
	return variable{lastWrite: newvc2(), lwDot: nil, frontier: make([]*dot, 0), current: 0, graph: myGraph{make([][]*dot, 0, 1000)}, races: make([]datarace, 0, 1000), history: make([]variableHistory, 0)}
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
		//t1 = newvc2().set(uint32(p.T1), 1)
		t1 = newT(p.T1)
	}

	lock = t1.vc.clone()               //Rel(x) = Th(i)
	t1.vc = t1.vc.add(uint32(p.T1), 1) //inc(Th(i),i)
	delete(t1.ls, p.T2)

	threads[p.T1] = t1
	locks[p.T2] = lock
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

	t1.ls[p.T2] = struct{}{}
	t1.vc = t1.vc.ssync(lock)  //Th(i) = Th(i) U Rel(x)
	t1.vc = t1.vc.add(p.T1, 1) //inc(Th(i),i)

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
		t2 = newT(p.T1)
	}

	t1.vc = t1.vc.add(p.T1, 1)
	t2.vc = t2.vc.add(p.T2, 1)

	t1.vc = t1.vc.ssync(t2.vc)
	t2.vc = t1.vc.clone()

	threads[p.T1] = t1
	threads[p.T2] = t2
}

var startDot = dot{int: 0}

var readCount uint32
var writeCount uint32

// func (v *variable) CreateDot(p *util.Item, vc vcepoch) (*dot, bool) {
// 	doubleFilter, ok := doubleFilterPerT[p.Thread]
// 	if !ok {
// 		doubleFilter = make(map[loc]*dot)
// 	}

// 	currLoc := loc{p.Ops[0].SourceRef, p.Ops[0].Line}
// 	newFE, ok := doubleFilter[currLoc]

// 	if !ok {
// 		v.current++
// 		newFE = &dot{v: vc.clone(), int: v.current, ev: p, write: p.Ops[0].Kind&util.WRITE > 0}
// 		doubleFilter[currLoc] = newFE
// 		doubleFilterPerT[p.Thread] = doubleFilter
// 		return newFE, true
// 	}
// 	return &dot{v: vc.clone(), int: -1, ev: p, write: p.Ops[0].Kind&util.WRITE > 0}, false
// }

var countnew = 0
var countold = 0

func intersect(a, b map[uint32]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; ok {
			return true
		}
	}
	return false
}

var countFP = 0

func (l *ListenerDataAccessReadReadDirectOp) Put(p *util.SyncPair) {
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

	//var newFE *dot

	if p.Write {
		//newFE, isNew := varstate.CreateDot(p.Ev, t1)

		varstate.current++
		//newFE := &dot{v: t1.clone(), int: varstate.current, ev: p.Ev, write: true}
		newFE := &dot{v: t1.vc.clone(), int: varstate.current, t: uint16(p.T1), sourceRef: p.Ev.Ops[0].SourceRef, line: uint16(p.Ev.Ops[0].Line), write: true, ls: make(map[uint32]struct{})}
		for k := range t1.ls { //copy lockset
			newFE.ls[k] = struct{}{}
		}

		countnew++
		newFrontier := make([]*dot, 0, len(varstate.frontier))
		connectTo := make([]*dot, 0)

		for _, f := range varstate.frontier {
			//k := f.v.get(f.ev.Thread)       //j#k
			//thi_at_j := t1.get(f.ev.Thread) //Th(i)[j]
			k := f.v.get(uint32(f.t))
			thi_at_j := t1.vc.get(uint32(f.t))

			if k > thi_at_j { //&& !intersect(f.ls, newFE.ls) {
				varstate.races = append(varstate.races, datarace{newFE, f}) //conc(x) = {(j#k,i#Th(i)[i]) | j#k ∈ RW(x) ∧ k > Th(i)[j]} ∪ conc(x)
				newFrontier = append(newFrontier, f)                        // RW(x) =  {j#k | j#k ∈ RW(x) ∧ k > Th(i)[j]}

				//varstate.addRace(newFE, f) //remove eventually
				// report.ReportRace(report.Location{File: f.ev.Ops[0].SourceRef, Line: f.ev.Ops[0].Line, W: f.write},
				// 	report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)
				b := report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
					report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)
				if b {
					b = intersect(f.ls, newFE.ls)
					if b {
						countFP++
					}
					fmt.Println(">>>", b, countFP)
				}
			} else if k < thi_at_j {
				connectTo = append(connectTo, f)
			}
		}

		varstate.updateGraph3(newFE, connectTo)

		newFrontier = append(newFrontier, newFE) // ∪{i#Th(i)[i]}
		varstate.frontier = newFrontier

		varstate.lastWrite = t1.vc.clone()
		varstate.lwDot = newFE
		t1.vc = t1.vc.add(p.T1, 1)

		//connect to artifical start dot if no connection exists
		// x, ok := varstate.graph[newFE.int]
		// if !ok {
		// 	x = append(x, &startDot)
		// 	varstate.graph[newFE.int] = x
		// }

		list := varstate.graph.get(newFE.int)
		if len(list) == 0 {
			list = append(list, &startDot)
			varstate.graph.add(newFE.int, list)
		}

	} else if p.Read {
		//newFE, isNew := varstate.CreateDot(p.Ev, t1)

		varstate.current++
		//newFE := &dot{int: varstate.current, ev: p.Ev, write: false}
		newFE := &dot{v: t1.vc.clone(), int: varstate.current, t: uint16(p.T1), sourceRef: uint32(p.Ev.Ops[0].SourceRef), line: uint16(p.Ev.Ops[0].Line), write: false, ls: make(map[uint32]struct{})}
		for k := range t1.ls { //copy lockset
			newFE.ls[k] = struct{}{}
		}
		//write read dependency race

		countnew++
		if varstate.lwDot != nil {
			//curVal := t1.get(varstate.lwDot.ev.Thread)
			//lwVal := varstate.lastWrite.get(varstate.lwDot.ev.Thread)
			curVal := t1.vc.get(uint32(varstate.lwDot.t))
			lwVal := varstate.lastWrite.get(uint32(varstate.lwDot.t))
			if lwVal > curVal { //&& !intersect(varstate.lwDot.ls, newFE.ls) {
				//varstate.addRace(newFE, varstate.lwDot)
				// report.ReportRace(report.Location{File: varstate.lwDot.ev.Ops[0].SourceRef, Line: varstate.lwDot.ev.Ops[0].Line, W: true},
				// 	report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, true, 0)
				b := report.ReportRace(report.Location{File: uint32(varstate.lwDot.sourceRef), Line: uint32(varstate.lwDot.line), W: true},
					report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, true, 0)
				if b {
					b = intersect(varstate.lwDot.ls, newFE.ls)
					if b {
						countFP++
					}
					fmt.Println(">>>", b, countFP)
				}
			}
		}

		//write-read sync
		t1.vc = t1.vc.ssync(varstate.lastWrite) //Th(i) = max(Th(i),L W (x))
		newFE.v = t1.vc.clone()

		newFrontier := make([]*dot, 0, len(varstate.frontier))
		connectTo := make([]*dot, 0)
		for _, f := range varstate.frontier {

			// k := f.v.get(f.ev.Thread)       //j#k
			// thi_at_j := t1.get(f.ev.Thread) //Th(i)[j]
			k := f.v.get(uint32(f.t))          //j#k
			thi_at_j := t1.vc.get(uint32(f.t)) //Th(i)[j]

			if k > thi_at_j { //&& !intersect(f.ls, newFE.ls) {

				newFrontier = append(newFrontier, f) // RW(x) =  {j]k | j]k ∈ RW(x) ∧ k > Th(i)[j]}

				if f.write {
					// report.ReportRace(report.Location{File: f.ev.Ops[0].SourceRef, Line: f.ev.Ops[0].Line, W: f.write},
					// 	report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)
					b := report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
						report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)
					if b {
						b = intersect(f.ls, newFE.ls)
						if b {
							countFP++
						}
						fmt.Println(">>>", b, countFP)
					}
					varstate.races = append(varstate.races, datarace{newFE, f}) //conc(x) = {(j#k,i]Th(i)[i]) | j#k ∈ RW(x) ∧ k > Th(i)[j]} ∪ conc(x)
					//varstate.addRace(newFE, f)
				}
			} else {
				if f.int > 0 {
					connectTo = append(connectTo, f)
				}
				if f.write {
					newFrontier = append(newFrontier, f)
				}
			}
		}

		countnew++
		varstate.updateGraph3(newFE, connectTo)

		newFrontier = append(newFrontier, newFE) // ∪{i#Th(i)[i]}
		varstate.frontier = newFrontier

		t1.vc = t1.vc.add(p.T1, 1) //inc(Th(i),i)

		//connect to artifical start dot if no connection exists

		list := varstate.graph.get(newFE.int)
		if len(list) == 0 {
			list = append(list, &startDot)
			varstate.graph.add(newFE.int, list)
		}

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

	//update states
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

	signalList[p.T2] = t1.vc.clone()

	t1.vc = t1.vc.add(uint32(p.T1), 1)

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
		t1.vc = t1.vc.ssync(t2)
		t1.vc = t1.vc.add(uint32(p.T1), 1)
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

var stats = false
var locations = make(map[string]struct{})

func (l *ListenerDataAccessLastTry) Put(p *util.SyncPair) {
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

	var newFE *dot

	if p.Write {
		varstate.current++
		//newFE = &dot{v: t1.clone(), int: varstate.current, ev: p.Ev, pos: len(varstate.history), write: true}
		newFE = &dot{v: t1.vc.clone(), int: varstate.current, t: uint16(p.T1), sourceRef: uint32(p.Ev.Ops[0].SourceRef), line: uint16(p.Ev.Ops[0].Line), write: true, pos: len(varstate.history), ls: make(map[uint32]struct{})}
		for k := range t1.ls { //copy lockset
			newFE.ls[k] = struct{}{}
		}
		//varstate.history = append(varstate.history, variableHistory{ev: p.Ev, isWrite: true, t: p.T1, c: t1.get(p.T1)})
		varstate.history = append(varstate.history, variableHistory{sourceRef: uint32(p.Ev.Ops[0].SourceRef), line: uint16(p.Ev.Ops[0].Line), isWrite: true, t: p.T1, c: t1.vc.get(p.T1), ls: newFE.ls})

		//	varstate.isWrite.Set(newFE.int)
		newFrontier := make([]*dot, 0, len(varstate.frontier))

		for _, f := range varstate.frontier {
			// k := f.v.get(f.ev.Thread)       //j#k
			// thi_at_j := t1.get(f.ev.Thread) //Th(i)[j]
			k := f.v.get(uint32(f.t))          //j#k
			thi_at_j := t1.vc.get(uint32(f.t)) //Th(i)[j]

			if k > thi_at_j { //&& !intersect(f.ls, newFE.ls) {
				varstate.races = append(varstate.races, datarace{newFE, f}) //conc(x) = {(j#k,i#Th(i)[i]) | j#k ∈ RW(x) ∧ k > Th(i)[j]} ∪ conc(x)
				newFrontier = append(newFrontier, f)                        // RW(x) =  {j#k | j#k ∈ RW(x) ∧ k > Th(i)[j]}

				// report.ReportRace(report.Location{File: f.ev.Ops[0].SourceRef, Line: f.ev.Ops[0].Line, W: f.write},
				// 	report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)
				report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
					report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)

				if stats {
					loc1 := fmt.Sprintf("%v:%v", f.sourceRef, f.line)
					loc2 := fmt.Sprintf("%v:%v", p.Ev.Ops[0].SourceRef, p.Ev.Ops[0].Line)

					locations[loc1] = struct{}{}
					locations[loc2] = struct{}{}
				}
			}
		}

		newFrontier = append(newFrontier, newFE)
		varstate.frontier = newFrontier

		varstate.lastWrite = t1.vc.clone()
		varstate.lwDot = newFE
		t1.vc = t1.vc.add(p.T1, 1)
	} else if p.Read {
		varstate.current++
		//newFE = &dot{int: varstate.current, ev: p.Ev, pos: len(varstate.history), write: false}
		newFE = &dot{v: t1.vc.clone(), int: varstate.current, t: uint16(p.T1), sourceRef: uint32(p.Ev.Ops[0].SourceRef), line: uint16(p.Ev.Ops[0].Line), write: false, pos: len(varstate.history), ls: make(map[uint32]struct{})}
		for k := range t1.ls { //copy lockset
			newFE.ls[k] = struct{}{}
		}
		if varstate.lwDot != nil {
			// curVal := t1.get(uint32(varstate.lwDot.ev.Thread))
			// lwVal := varstate.lastWrite.get(uint32(varstate.lwDot.ev.Thread))
			curVal := t1.vc.get(uint32(varstate.lwDot.t))
			lwVal := varstate.lastWrite.get(uint32(varstate.lwDot.t))

			if lwVal > curVal { //&& !intersect(varstate.lwDot.ls, newFE.ls) {
				// report.ReportRace(report.Location{File: varstate.lwDot.ev.Ops[0].SourceRef, Line: varstate.lwDot.ev.Ops[0].Line, W: true},
				// 	report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, true, 0)
				report.ReportRace(report.Location{File: uint32(varstate.lwDot.sourceRef), Line: uint32(varstate.lwDot.line), W: true},
					report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, true, 0)

				if stats {
					loc1 := fmt.Sprintf("%v:%v", varstate.lwDot.sourceRef, varstate.lwDot.line)
					loc2 := fmt.Sprintf("%v:%v", p.Ev.Ops[0].SourceRef, p.Ev.Ops[0].Line)

					locations[loc1] = struct{}{}
					locations[loc2] = struct{}{}
				}
			}
		}

		t1.vc = t1.vc.ssync(varstate.lastWrite) //Th(i) = max(Th(i),L W (x))
		newFE.v = t1.vc.clone()

		//varstate.history = append(varstate.history, newFE)
		//varstate.history = append(varstate.history, variableHistory{ev: p.Ev, isWrite: false, t: p.T1, c: t1.get(p.T1)})
		varstate.history = append(varstate.history, variableHistory{sourceRef: uint32(p.Ev.Ops[0].SourceRef), line: uint16(p.Ev.Ops[0].Line), isWrite: false, t: p.T1, c: t1.vc.get(p.T1), ls: newFE.ls})

		newFrontier := make([]*dot, 0, len(varstate.frontier))
		for _, f := range varstate.frontier {
			// k := f.v.get(uint32(f.ev.Thread))       //j#k
			// thi_at_j := t1.get(uint32(f.ev.Thread)) //Th(i)[j]
			k := f.v.get(uint32(f.t))          //j#k
			thi_at_j := t1.vc.get(uint32(f.t)) //Th(i)[j]

			if k > thi_at_j { //&& !intersect(f.ls, newFE.ls) {
				newFrontier = append(newFrontier, f) // RW(x) =  {j#k | j#k ∈ RW(x) ∧ k > Th(i)[j]}

				if f.write {
					varstate.races = append(varstate.races, datarace{newFE, f}) //conc(x) = {(j#k,i#Th(i)[i]) | j#k ∈ RW(x) ∧ k > Th(i)[j]} ∪ conc(x)
					// report.ReportRace(report.Location{File: f.ev.Ops[0].SourceRef, Line: f.ev.Ops[0].Line, W: f.write},
					// 	report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)
					report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
						report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)

					if stats {
						loc1 := fmt.Sprintf("%v:%v", f.sourceRef, f.line)
						loc2 := fmt.Sprintf("%v:%v", p.Ev.Ops[0].SourceRef, p.Ev.Ops[0].Line)

						locations[loc1] = struct{}{}
						locations[loc2] = struct{}{}
					}
				}
			} else if f.write {
				newFrontier = append(newFrontier, f)
			}
		}
		newFrontier = append(newFrontier, newFE)
		varstate.frontier = newFrontier

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

	threads[p.T1] = t1
	variables[p.T2] = varstate
}

func (l *ListenerPostProcess2) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}

	idx := 1
	//end := len(variables)
	for _, v := range variables {
		//fmt.Printf("\r%v/%v - %v", idx, end, len(v.history))
		for _, race := range v.races {
			raceDot := race.d1
			raceIsWrite := race.d1.write //v.isWrite.Get(race.d1.int)
			//raceThread := raceDot.ev.Thread
			raceThread := uint32(raceDot.t)

			for j := race.d2.pos; j >= 0; j-- {
				prevDot := v.history[j]
				if !raceIsWrite && !prevDot.isWrite || raceThread == prevDot.t {
					continue
				}

				raceVal := raceDot.v.get(prevDot.t)
				prevVal := prevDot.c   //prevDot.v.get(prevDot.ev.Thread)
				if prevVal > raceVal { //&& !intersect(raceDot.ls, prevDot.ls) {
					// report.ReportRace(report.Location{File: prevDot.ev.Ops[0].SourceRef, Line: prevDot.ev.Ops[0].Line, W: prevDot.isWrite},
					// 	report.Location{File: raceDot.ev.Ops[0].SourceRef, Line: raceDot.ev.Ops[0].Line, W: raceIsWrite}, false, 1)
					// report.ReportRace(report.Location{File: uint32(prevDot.sourceRef), Line: uint32(prevDot.line), W: prevDot.isWrite},
					// 	report.Location{File: raceDot.ev.Ops[0].SourceRef, Line: raceDot.ev.Ops[0].Line, W: raceIsWrite}, false, 1)
					report.ReportRace(report.Location{File: uint32(prevDot.sourceRef), Line: uint32(prevDot.line), W: prevDot.isWrite},
						report.Location{File: uint32(raceDot.sourceRef), Line: uint32(raceDot.line), W: raceIsWrite}, false, 1)
				}

			}
		}
		idx++
	}
}

func (l *ListenerPostProcess3) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}

	distinctLocation := 0
	maxDistinctLocations := 0
	//sameLocation := 0
	checkedRaces := 0
	alreadyReported := 0
	newLocation := 0

	idx := 1
	//end := len(variables)
	for _, v := range variables {
		//fmt.Printf("\r%v/%v - %v", idx, end, len(v.history))
		for _, race := range v.races {
			raceDot := race.d1
			raceIsWrite := race.d1.write //v.isWrite.Get(race.d1.int)
			//raceThread := raceDot.ev.Thread
			raceThread := uint32(raceDot.t)

			//partnerLoc := fmt.Sprintf("%v:%v", race.d2.ev.Ops[0].SourceRef, race.d2.ev.Ops[0].Line)
			localDistinct := 0
			checkedRaces++

			for j := race.d2.pos; j >= 0; j-- {
				prevDot := v.history[j]
				if !raceIsWrite && !prevDot.isWrite || raceThread == prevDot.t {
					continue
				}

				raceVal := raceDot.v.get(prevDot.t)
				prevVal := prevDot.c //prevDot.v.get(prevDot.ev.Thread)
				if prevVal > raceVal {
					// b := report.ReportRace(report.Location{File: uint32(prevDot.sourceRef), Line: uint32(prevDot.line), W: prevDot.isWrite},
					// 	report.Location{File: raceDot.ev.Ops[0].SourceRef, Line: raceDot.ev.Ops[0].Line, W: raceIsWrite}, false, 1)
					b := report.ReportRace(report.Location{File: uint32(prevDot.sourceRef), Line: uint32(prevDot.line), W: prevDot.isWrite},
						report.Location{File: uint32(raceDot.sourceRef), Line: uint32(raceDot.line), W: raceIsWrite}, false, 1)

					if b { // not reported
						currLoc := fmt.Sprintf("%v:%v", prevDot.sourceRef, prevDot.line)
						if _, ok := locations[currLoc]; !ok {
							newLocation++
						}

					}
				}
			}
			if localDistinct > maxDistinctLocations {
				maxDistinctLocations = localDistinct
			}
		}
		idx++
	}
	fmt.Printf("NewLocations(ALL):%v\nDistinctLocations(ALL):%v\nAlreadyReported(ALL):%v\nMaxDistinctLoc:%v\nAvgDistinctLoc:%v\n", newLocation, distinctLocation,
		alreadyReported, maxDistinctLocations, float64(distinctLocation)/float64(checkedRaces))
}

func (l *ListenerPostProcess) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}

	fmt.Println("@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@")
	maxGraph := 0
	countGraph := 0.0
	sumGraph := 0.0
	minGraph := ^uint(0)

	lengths := make(sort.IntSlice, 0, len(variables))

	for _, v := range variables {
		sumGraph += float64(len(v.graph.ds))
		countGraph++
		if len(v.graph.ds) > maxGraph {
			maxGraph = len(v.graph.ds)
		}
		lengths = append(lengths, len(v.graph.ds))

		if uint(len(v.graph.ds)) < minGraph {
			minGraph = uint(len(v.graph.ds))
		}
		v.detectGraphRaces()
	}

	lengths.Sort()
	size := len(lengths)
	if size%2 == 0 {
		fmt.Println("MEDIAN:", (lengths[size/2-1]+lengths[size/2])/2)
	} else {
		fmt.Println("MEDIAN:", lengths[size/2])
	}

	fmt.Println("MAxDist:", maxDistance)
	fmt.Println("AvgDist:", float64(distSum)/float64(distCount))
	fmt.Println("MinDist:", minDistance)
	fmt.Println("DistCount:", distCount, distSum)

	fmt.Println("NewDots:", countnew)
	fmt.Println("OldDots:", countold)

	fmt.Println("FALSE POSITIVES:", countFP)

	fmt.Println("GRAPH DATA")
	fmt.Println("MAX GRAPH:", maxGraph)
	fmt.Println("MIN GRAPH:", minGraph)
	fmt.Println("AVG GRAPH:", sumGraph/countGraph)
	fmt.Println("#GRAPHS:", len(variables))
}

// func (v *variable) updateGraph2(nf *dot, of *dot) {
// 	x := v.graph[nf.int]

// 	if x == nil {
// 		x = []*dot{of}
// 		v.graph[nf.int] = x
// 		return
// 	}

// 	for _, y := range x {
// 		if of.int == y.int {
// 			return
// 		}
// 	}

// 	x = append(x, of)
// 	v.graph[nf.int] = x
// }
func (v *variable) updateGraph3(nf *dot, of []*dot) {
	//v.graph[nf.int] = of
	v.graph.add(nf.int, of)
}

type loc struct {
	srcRef uint32
	line   uint32
}

var doubleFilterPerT = make(map[uint32]map[loc]*dot)

// func (v *variable) updateGraph4(nf *dot, of []*dot) {
// 	doubleFilter, ok := doubleFilterPerT[nf.ev.Thread]
// 	if !ok {
// 		doubleFilter = make(map[loc]*dot)
// 	}
// 	currLoc := loc{nf.ev.Ops[0].SourceRef, nf.ev.Ops[0].Line}

// 	if xf, ok := doubleFilter[currLoc]; !ok {
// 		doubleFilter[currLoc] = nf
// 		doubleFilterPerT[nf.ev.Thread] = doubleFilter
// 		v.graph.add(nf.int, of)
// 	} else {
// 		nf.int = xf.int
// 	}
// }

var chainSum uint64
var sumChains uint64
var maxChain uint64
var zeroChain uint64
var greaterZeroChain uint64

func (v *variable) detectGraphRaces() {
	if len(v.races) == 0 {
		return
	}

	// steps := 0
	// stepBarrier := (len(v.races) / 10) + 1
	visMap := make(map[int]*bitset.BitSet)
	//cache := make(map[int][]*dot)
	for _, dr := range v.races {
		// steps++
		// if steps%stepBarrier == 0 {
		// 	fmt.Printf("\r%v/%v", i+1, len(v.races))
		// }

		d := dr.d1

		visited, ok := visMap[d.int]
		if !ok {
			visited = &bitset.BitSet{}
		}
		// if !v.isWrite.Get(dr.d1.int) && !v.isWrite.Get(dr.d2.int) {
		// 	v.findRacesReadOp(dr.d1, dr.d2, cache)
		// 	//v.findRaces(dr.d1, dr.d2, visited)
		// } else {
		v.findRaces(dr.d1, dr.d2, visited, 0)
		//}

		visMap[d.int] = visited
	}
}

func (v *variable) calcWriteReach(acc *dot) []*dot {
	reach := make([]*dot, 0)

	//for _, d := range v.graph[acc.int] {
	for _, d := range v.graph.get(acc.int) {
		if d.write {
			reach = append(reach, d)
		}
		reach = append(reach, v.calcWriteReach(d)...)
	}
	return reach
}

func (v *variable) calcWriteReach2(acc *dot, cache map[int][]*dot) []*dot {
	if wr, ok := cache[acc.int]; ok {
		return wr
	}

	reach := make([]*dot, 0)

	//for _, d := range v.graph[acc.int] {
	for _, d := range v.graph.get(acc.int) {
		if d.write {
			reach = append(reach, d)
		}
		reach = append(reach, v.calcWriteReach2(d, cache)...)
	}
	cache[acc.int] = reach
	return reach
}

// func (v *variable) findRacesReadOp(raceR, prevR *dot, cache map[int][]*dot) {
// 	// var writeReach []*dot
// 	// if wr, ok := cache[prevR.int]; ok {
// 	// 	writeReach = wr
// 	// } else {
// 	// 	writeReach = v.calcWriteReach(prevR)
// 	// 	cache[prevR.int] = writeReach
// 	// }
// 	writeReach := v.calcWriteReach2(prevR, cache)

// 	for _, d := range writeReach {
// 		if d.int == 0 || !d.write {
// 			continue
// 		}

// 		dVal := d.v.get(uint32(d.ev.Thread))
// 		raVal := raceR.v.get(uint32(d.ev.Thread))

// 		if dVal > raVal /*&& v.isNewRace(raceR, d)*/ {
// 			if raceR.int > d.int { //raceR after d
// 				report.ReportRace(
// 					report.Location{File: d.ev.Ops[0].SourceRef, Line: d.ev.Ops[0].Line, W: d.write},
// 					report.Location{File: raceR.ev.Ops[0].SourceRef, Line: raceR.ev.Ops[0].Line, W: raceR.write},
// 					false, 1)
// 			} else {
// 				fmt.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!FOOOBAR") //should never happen!
// 				report.ReportRace(
// 					report.Location{File: raceR.ev.Ops[0].SourceRef, Line: raceR.ev.Ops[0].Line, W: raceR.write},
// 					report.Location{File: d.ev.Ops[0].SourceRef, Line: d.ev.Ops[0].Line, W: d.write},
// 					false, 1)
// 			}
// 			//v.addRace(raceR, d)
// 		}
// 	}
// }

var maxDistance int
var minDistance = int((^uint(0)) >> 1)
var distCount int
var distSum int

//second is the dot that triggered the race
func (v *variable) findRaces(raceAcc, prevAcc *dot, visited *bitset.BitSet, level uint64) {
	if visited.Get(prevAcc.int) {
		return
	}
	visited.Set(prevAcc.int)

	//for _, d := range v.graph[prevAcc.int] {
	for _, d := range v.graph.get(prevAcc.int) {
		if d.int == 0 {
			continue
		}

		// dVal := d.v.get((d.ev.Thread))
		// raVal := raceAcc.v.get((d.ev.Thread))
		dVal := d.v.get(uint32(d.t))
		raVal := raceAcc.v.get(uint32(d.t))

		if dVal > raVal { // && !intersect(d.ls, raceAcc.ls) /*&& v.isNewRace(raceAcc, d)*/ {

			// report.ReportRace(report.Location{File: d.ev.Ops[0].SourceRef, Line: d.ev.Ops[0].Line, W: d.write},
			// 	report.Location{File: raceAcc.ev.Ops[0].SourceRef, Line: raceAcc.ev.Ops[0].Line, W: raceAcc.write}, false, 1)
			b := report.ReportRace(report.Location{File: uint32(d.sourceRef), Line: uint32(d.line), W: d.write},
				report.Location{File: uint32(raceAcc.sourceRef), Line: uint32(raceAcc.line), W: raceAcc.write}, false, 1)
			if b {
				b = intersect(d.ls, raceAcc.ls)
				if b {
					countFP++
				} else {
					distCount++
					distSum += raceAcc.int - d.int
					if (raceAcc.int - d.int) > maxDistance {
						maxDistance = raceAcc.int - d.int
					}

					if (raceAcc.int - d.int) < minDistance {
						minDistance = raceAcc.int - d.int
					}
				}
				fmt.Println(">>>", b, countFP)

			}

			//	v.addRace(raceAcc, d)

			v.findRaces(raceAcc, d, visited, level+1)
		}
	}

}

func (v *variable) find(current, toFind *dot) bool {
	if current.int == toFind.int {
		return true
	}

	if current.int < toFind.int {
		return false
	}

	//for _, d := range v.graph[current.int] {
	for _, d := range v.graph.get(current.int) {
		if v.find(d, toFind) {
			return true
		}
	}

	return false
}
