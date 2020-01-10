package tsanee

import (
	"github.com/xojoc/bitset"

	"../../util"
	"../analysis"
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

	locks = make(map[uint32]lock)
	threads = make(map[uint32]thread)
	signalList = make(map[uint32]vcepoch)
	variables = make(map[uint32]variable)
	volatiles = make(map[uint32]vcepoch)
	notifies = make(map[uint32]vcepoch)

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

	algos.RegisterDetector("tsanee", &EventCollector{listeners})

}

var threads map[uint32]thread
var locks map[uint32]lock
var signalList map[uint32]vcepoch
var variables map[uint32]variable
var volatiles map[uint32]vcepoch
var notifies map[uint32]vcepoch

type thread struct {
	vc         vcepoch
	hb         vcepoch
	ls         map[uint32]struct{}
	strongSync uint32
}

type lock struct {
	rel vcepoch
	hb  vcepoch
}

func newT(id uint32) thread {
	return thread{vc: newvc2().set(id, 1), hb: newvc2().set(id, 1), ls: make(map[uint32]struct{})}
}

func newL() lock {
	return lock{rel: newvc2(), hb: newvc2()}
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
	lwLocks   map[uint32]struct{}
	lwDot     *dot
	current   int
}

func newVar() variable {
	return variable{lastWrite: newvc2(), lwDot: nil, frontier: make([]*dot, 0),
		current: 0, graph: newGraph(), lwLocks: make(map[uint32]struct{})}
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
	v         vcepoch
	ls        map[uint32]struct{}
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

const maxsize = 25

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
		lock = newL()
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	lock.hb = lock.hb.ssync(t1.hb)
	delete(t1.ls, p.T2)

	t1.vc = t1.vc.add(p.T1, 1) //inc(Th(i),i)
	t1.hb = t1.hb.add(p.T1, 1)

	if t1.strongSync == p.T2 {
		t1.strongSync = 0
	} else if t1.strongSync > 0 {
		lock.rel = lock.rel.ssync(t1.hb)
	}

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
		lock = newL()
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	t1.vc = t1.vc.ssync(lock.rel) //Th(i) = Th(i) U Rel(x)
	t1.hb = t1.hb.ssync(lock.hb)  // Th(i)_hb = Th(i)_hb U Rel(x)_hb (hb synchro)
	t1.ls[p.T2] = struct{}{}

	t1.vc = t1.vc.add(p.T1, 1)
	t1.hb = t1.hb.add(p.T1, 1)

	threads[p.T1] = t1
	locks[p.T2] = lock
}

var startDot = dot{int: 0}

//intersect returns true if the two sets have at least one element in common
func intersect(a, b map[uint32]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; ok {
			return true
		}
	}
	return false
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
		varstate = newVar()
	}

	if p.Write {
		varstate.current++
		newFE := &dot{v: t1.vc.clone(), int: varstate.current, t: uint16(p.T1),
			sourceRef: p.Ev.Ops[0].SourceRef, line: uint16(p.Ev.Ops[0].Line),
			write: true, ls: make(map[uint32]struct{}), pos: p.Ev.LocalIdx}
		for k := range t1.ls { //copy lockset
			newFE.ls[k] = struct{}{}
		}

		newFrontier := make([]*dot, 0, len(varstate.frontier))
		connectTo := make([]*dot, 0)

		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))
			thi_at_j := t1.vc.get(uint32(f.t))

			if k > thi_at_j {
				newFrontier = append(newFrontier, f) // RW(x) =  {j#k | j#k ∈ RW(x) ∧ k > Th(i)[j]}

				if !intersect(newFE.ls, f.ls) {
					report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
						report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)

				}
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
		varstate.lwLocks = make(map[uint32]struct{})
		for k := range t1.ls {
			varstate.lwLocks[k] = struct{}{}
		}
		varstate.lwDot = newFE
		//t1.vc = t1.vc.add(p.T1, 1)

		list, ok := varstate.graph.get(newFE.int)
		if ok && len(list) == 0 {
			list = append(list, &startDot)
			varstate.graph.add(newFE, list)
		}

	} else if p.Read {
		varstate.current++
		newFE := &dot{v: t1.vc.clone(), int: varstate.current, t: uint16(p.T1),
			sourceRef: uint32(p.Ev.Ops[0].SourceRef), line: uint16(p.Ev.Ops[0].Line),
			write: false, ls: make(map[uint32]struct{}), pos: p.Ev.LocalIdx}
		for k := range t1.ls { //copy lockset
			newFE.ls[k] = struct{}{}
		}

		//locks accumulate lastWrite vcs for lockset of t
		if varstate.lwDot != nil {
			for k := range t1.ls {
				lk := locks[k]
				lk.rel = lk.rel.ssync(varstate.lastWrite)

				//new part for this implementation! different on purpose
				//order critical sections if the last write is inside the
				//same lock as the current read
				if _, ok := varstate.lwLocks[k]; ok { //ab hier weis ich das strong sync noetig ist!
					t1.vc = t1.vc.ssync(lk.hb)
					if t1.strongSync == 0 { // dont let it be overwritten all the time
						t1.strongSync = k //buggy, needs to be the most outer lock here its some lock in ls of thread
					}
				}

				locks[k] = lk
			}
		}

		//write read dependency race
		if varstate.lwDot != nil {
			curVal := t1.vc.get(uint32(varstate.lwDot.t))
			lwVal := varstate.lastWrite.get(uint32(varstate.lwDot.t))
			if lwVal > curVal {
				if !intersect(newFE.ls, varstate.lwDot.ls) {
					report.ReportRace(report.Location{File: uint32(varstate.lwDot.sourceRef), Line: uint32(varstate.lwDot.line), W: true},
						report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, true, 0)

				}
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

			if k > thi_at_j {

				newFrontier = append(newFrontier, f) // RW(x) =  {j]k | j]k ∈ RW(x) ∧ k > Th(i)[j]}

				if f.write {
					if !intersect(newFE.ls, f.ls) {
						report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
							report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)

					}

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
	}
	t1.vc = t1.vc.add(p.T1, 1) //inc(Th(i),i)
	t1.hb = t1.hb.add(p.T1, 1)
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

	if t1.strongSync > 0 {
		signalList[p.T2] = t1.vc.clone().ssync(t1.hb)
	}

	t1.vc = t1.vc.add(p.T1, 1)
	t1.hb = t1.hb.add(p.T1, 1)
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
		t1.hb = t1.hb.add(p.T1, 1)
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

	if t1.strongSync == 0 {
		vc = vc.ssync(t1.vc)
	} else {
		vc = vc.ssync(t1.vc.clone().ssync(t1.hb))
	}
	t1.vc = t1.vc.add(p.T1, 1)
	t1.hb = t1.hb.add(p.T1, 1)
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
		t1.hb = t1.hb.add(p.T1, 1)

		if t1.strongSync == 0 {
			vc = t1.vc.clone()
		} else {
			vc = vc.ssync(t1.vc.clone().ssync(t1.hb))
		}

		threads[p.T1] = t1
		notifies[p.T2] = vc
	}

}

func (l *ListenerPostProcess) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}
}

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
			if !intersect(raceAcc.ls, d.ls) {
				report.ReportRace(report.Location{File: uint32(d.sourceRef), Line: uint32(d.line), W: d.write},
					report.Location{File: uint32(raceAcc.sourceRef), Line: uint32(raceAcc.line), W: raceAcc.write}, false, 1)

				// if b {
				// 	fmt.Println("LS's", raceAcc.ls, d.ls, intersect(raceAcc.ls, d.ls))
				// }
			}
			v.findRaces(raceAcc, d, visited, level+1)
		}
	}

}
