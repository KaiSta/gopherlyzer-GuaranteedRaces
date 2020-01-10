package fsshbee

import (
	"fmt"

	"github.com/xojoc/bitset"

	"../../util"
	algos "../analysis"
	"../report"
	"../traceReplay"
)

type ListenerAsyncSnd struct{}
type ListenerAsyncRcv struct{}
type ListenerDataAccess struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}
type ListenerNT struct{}
type ListenerNTWT struct{}
type ListenerPostProcess struct{}

type EventCollector struct {
	listeners []traceReplay.EventListener
}

func (l *EventCollector) Put(p *util.SyncPair) {
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
		&ListenerDataAccess{},
		//&ListenerDataAccessLastTry{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}

	algos.RegisterDetector("fsshbee", &EventCollector{listeners})

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
	//races []datarace
	//	history   []variableHistory
	frontier  []*dot
	graph     *fsGraph
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

func newVar() variable {
	return variable{lastWrite: newvc2(), lwDot: nil, frontier: make([]*dot, 0), current: 0, graph: newGraph()}
}

type dataRace struct {
	raceAcc int
	prevAcc int
}

type variableHistory struct {
	sourceRef uint32
	t         uint32
	c         uint32
	line      uint16
	isWrite   bool
}

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

const maxsize = 25 // 510

type node struct {
	neighbors []int
	d         *dot
}
type fsGraph struct {
	ds []node
}

func newGraph() *fsGraph {
	return &fsGraph{ds: make([]node, 0)}
}

func (g *fsGraph) add(nd *dot, dots []*dot) {
	if len(g.ds) >= maxsize {
		g.ds = g.ds[1:] //remove first element by shifting the array one to the left
	}

	newNode := node{d: nd}
	for _, d := range dots {
		newNode.neighbors = append(newNode.neighbors, d.int) //only the ints, not the dots otherwise the dots would live on in the memory
	}
	g.ds = append(g.ds, newNode)
}

func (v *variable) updateGraph3(nf *dot, of []*dot) {
	v.graph.add(nf, of)
}

func (g *fsGraph) get(dID int) ([]*dot, bool) {
	left, right := 0, len(g.ds)-1

	for left <= right {
		mid := left + ((right - left) / 2)

		if g.ds[mid].d.int == dID {
			dots := make([]*dot, 0, len(g.ds[mid].neighbors))
			for _, n := range g.ds[mid].neighbors {
				if d := g.find_internal(n); d != nil {
					//neighbour dot still in graph
					dots = append(dots, d)
				}
			}
			return dots, true
		}

		if g.ds[mid].d.int > dID {
			right = mid - 1
		} else {
			left = mid + 1
		}

	}

	return nil, false
}

func (g *fsGraph) find_internal(dID int) *dot {
	left, right := 0, len(g.ds)-1

	for left <= right {
		mid := left + ((right - left) / 2)

		if g.ds[mid].d.int == dID {
			return g.ds[mid].d
		}

		if g.ds[mid].d.int > dID {
			right = mid - 1
		} else {
			left = mid + 1
		}

	}

	return nil
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

	t1.vc = t1.vc.ssync(lock) //Th(i) = Th(i) U Rel(x)
	t1.ls[p.T2] = struct{}{}

	threads[p.T1] = t1
}

var startDot = dot{int: 0}

func intersect(a, b map[uint32]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; ok {
			return true
		}
	}
	return false
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
		varstate = newVar()
	}

	if p.Write {
		varstate.current++
		newFE := &dot{v: t1.vc.clone(), int: varstate.current, t: uint16(p.T1),
			sourceRef: p.Ev.Ops[0].SourceRef, line: uint16(p.Ev.Ops[0].Line),
			write: true, ls: make(map[uint32]struct{})}
		for k := range t1.ls {
			newFE.ls[k] = struct{}{}
		}

		newFrontier := make([]*dot, 0, len(varstate.frontier))
		connectTo := make([]*dot, 0)

		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))
			thi_at_j := t1.vc.get(uint32(f.t))

			if k > thi_at_j { //&& !intersect(f.ls, newFE.ls) {
				//varstate.races = append(varstate.races, datarace{newFE, f}) //conc(x) = {(j#k,i#Th(i)[i]) | j#k ∈ RW(x) ∧ k > Th(i)[j]} ∪ conc(x)
				newFrontier = append(newFrontier, f) // RW(x) =  {j#k | j#k ∈ RW(x) ∧ k > Th(i)[j]}

				if !intersect(f.ls, newFE.ls) {
					report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
						report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)
				}
				// if b {
				// 	b = intersect(f.ls, newFE.ls)
				// 	if b {
				// 		countFP++
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }

				visited := &bitset.BitSet{}
				varstate.findRaces(newFE, f, visited, 0)

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

		list, ok := varstate.graph.get(newFE.int)
		if ok && len(list) == 0 {
			list = append(list, &startDot)
			varstate.graph.add(newFE, list)
		}

	} else if p.Read {
		varstate.current++
		newFE := &dot{v: t1.vc.clone(), int: varstate.current, t: uint16(p.T1),
			sourceRef: uint32(p.Ev.Ops[0].SourceRef), line: uint16(p.Ev.Ops[0].Line),
			write: false, ls: make(map[uint32]struct{})}
		for k := range t1.ls {
			newFE.ls[k] = struct{}{}
		}

		//write read dependency race
		if varstate.lwDot != nil {
			curVal := t1.vc.get(uint32(varstate.lwDot.t))
			lwVal := varstate.lastWrite.get(uint32(varstate.lwDot.t))
			if lwVal > curVal { // && !intersect(varstate.lwDot.ls, newFE.ls) {

				if !intersect(varstate.lwDot.ls, newFE.ls) {
					report.ReportRace(report.Location{File: uint32(varstate.lwDot.sourceRef), Line: uint32(varstate.lwDot.line), W: true},
						report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, true, 0)
				}

				// if b {
				// 	b = intersect(varstate.lwDot.ls, newFE.ls)
				// 	if b {
				// 		countFP++
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }
			}
		}

		//write-read sync
		t1.vc = t1.vc.ssync(varstate.lastWrite) //Th(i) = max(Th(i),L W (x))
		newFE.v = t1.vc.clone()

		newFrontier := make([]*dot, 0, len(varstate.frontier))
		connectTo := make([]*dot, 0)
		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))          //j#k
			thi_at_j := t1.vc.get(uint32(f.t)) //Th(i)[j]

			if k > thi_at_j { //&& !intersect(newFE.ls, f.ls) {

				newFrontier = append(newFrontier, f) // RW(x) =  {j]k | j]k ∈ RW(x) ∧ k > Th(i)[j]}

				if f.write {
					if !intersect(f.ls, newFE.ls) {
						report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
							report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)
					}
					// if b {
					// 	b = intersect(f.ls, newFE.ls)
					// 	if b {
					// 		countFP++
					// 	}
					// 	fmt.Println(">>>", b, countFP)
					// }
					//varstate.races = append(varstate.races, datarace{newFE, f}) //conc(x) = {(j#k,i]Th(i)[i]) | j#k ∈ RW(x) ∧ k > Th(i)[j]} ∪ conc(x)

					visited := &bitset.BitSet{}
					varstate.findRaces(newFE, f, visited, 0)
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

		varstate.updateGraph3(newFE, connectTo)

		newFrontier = append(newFrontier, newFE) // ∪{i#Th(i)[i]}
		varstate.frontier = newFrontier

		t1.vc = t1.vc.add(p.T1, 1) //inc(Th(i),i)

		//connect to artifical start dot if no connection exists

		list, ok := varstate.graph.get(newFE.int)
		if ok && len(list) == 0 {
			list = append(list, &startDot)
			varstate.graph.add(newFE, list)
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

	t1.vc = t1.vc.add(p.T1, 1)

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
		t1.vc = t1.vc.add(p.T1, 1)
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

func (l *ListenerPostProcess) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}

	fmt.Println("@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@")

	// for _, v := range variables {
	// 	v.detectGraphRaces()
	// }

	fmt.Println("False Positives:", countFP)

	// fmt.Println("MAxDist:", maxDistance)
	// fmt.Println("AvgDist:", float64(distSum)/float64(distCount))

	// fmt.Println("NewDots:", countnew)
	// fmt.Println("OldDots:", countold)
}

// func (v *variable) detectGraphRaces() {
// 	if len(v.races) == 0 {
// 		return
// 	}

// 	// steps := 0
// 	// stepBarrier := (len(v.races) / 10) + 1
// 	visMap := make(map[int]*bitset.BitSet)
// 	//cache := make(map[int][]*dot)
// 	for _, dr := range v.races {
// 		// steps++
// 		// if steps%stepBarrier == 0 {
// 		// 	fmt.Printf("\r%v/%v", i+1, len(v.races))
// 		// }

// 		d := dr.d1

// 		visited, ok := visMap[d.int]
// 		if !ok {
// 			visited = &bitset.BitSet{}
// 		}
// 		// if !v.isWrite.Get(dr.d1.int) && !v.isWrite.Get(dr.d2.int) {
// 		// 	v.findRacesReadOp(dr.d1, dr.d2, cache)
// 		// 	//v.findRaces(dr.d1, dr.d2, visited)
// 		// } else {
// 		v.findRaces(dr.d1, dr.d2, visited, 0)
// 		//}

// 		visMap[d.int] = visited
// 	}
// }

func (v *variable) findRaces(raceAcc, prevAcc *dot, visited *bitset.BitSet, level uint64) {
	if visited.Get(prevAcc.int) {
		return
	}
	visited.Set(prevAcc.int)

	list, ok := v.graph.get(prevAcc.int)
	if !ok {
		return
	}
	for _, d := range list {
		if d.int == 0 {
			continue
		}

		dVal := d.v.get(uint32(d.t))
		raVal := raceAcc.v.get(uint32(d.t))

		if dVal > raVal {
			if !intersect(d.ls, raceAcc.ls) {
				report.ReportRace(report.Location{File: uint32(d.sourceRef), Line: uint32(d.line), W: d.write},
					report.Location{File: uint32(raceAcc.sourceRef), Line: uint32(raceAcc.line), W: raceAcc.write}, false, 1)
			}
			// if b {
			// 	b = intersect(d.ls, raceAcc.ls)
			// 	if b {
			// 		countFP++
			// 	} else {
			// 		fmt.Println(raceAcc.int, d.int)
			// 		fmt.Println(raceAcc.int - d.int)
			// 	}
			// 	fmt.Println(">>>", b, countFP)
			// }

			v.findRaces(raceAcc, d, visited, level+1)
		}
	}

}
