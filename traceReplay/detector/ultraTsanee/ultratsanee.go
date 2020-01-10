package ultratsanee

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

var steps int
var stepBarrier = 1000000

func (l *EventCollector) Put(p *util.SyncPair) {
	for _, l := range l.listeners {
		l.Put(p)
	}

	steps++
	if steps%stepBarrier == 0 {
		checkOpenRaces()
	}
}

var maxOpenRaces int

func checkOpenRaces() {
	if len(openRaces) > 0 {
		fmt.Println("!!!!", len(openRaces))
		if len(openRaces) > maxOpenRaces {
			maxOpenRaces = len(openRaces)
		}
	}
	newOpenRaces := make([]datarace, 0, len(openRaces))
	for _, race := range openRaces {

		if checkRace(race.ols, race.d1, race.d2, false, 0, race.varid) { //race complete!
			//delete(openRaces, k)
		} else {
			newOpenRaces = append(newOpenRaces, race)
		}

	}
	if len(openRaces) > 0 {
		fmt.Println("REMOVED", len(openRaces)-len(newOpenRaces), "ELEMENTS")
	}
	openRaces = newOpenRaces
}

func Init() {

	locks = make(map[uint32]lock)
	threads = make(map[uint32]thread)
	signalList = make(map[uint32]vcepoch)
	variables = make(map[uint32]variable)
	volatiles = make(map[uint32]vcepoch)
	notifies = make(map[uint32]vcepoch)
	//openRaces = make(map[int]datarace)
	openRaces = make([]datarace, 0)

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

	algos.RegisterDetector("ultratsanee", &EventCollector{listeners})

}

var threads map[uint32]thread
var locks map[uint32]lock
var signalList map[uint32]vcepoch
var variables map[uint32]variable
var volatiles map[uint32]vcepoch
var notifies map[uint32]vcepoch

var openRaceCounter int

//var openRaces map[int]datarace
var openRaces []datarace

type thread struct {
	vc vcepoch
	//	hb vcepoch
	//ls         map[uint32]struct{}
	ls         []uint32
	lsboo      map[uint32]bool
	ols        map[uint32]int
	posAqRl    map[uint32]int
	strongSync uint32
}

type lock struct {
	rel   vcepoch
	hb    vcepoch
	count int
	//rels  map[int]vcepoch
	rels    []vcepoch
	acqRel  []acqRelPair
	ols     map[uint32]int
	acq     epoch
	nextAcq epoch
}

func newT(id uint32) thread {
	return thread{vc: newvc2().set(id, 1) /*hb: newvc2().set(id, 1),*/, ls: make([]uint32, 0) /*make(map[uint32]struct{})*/, lsboo: make(map[uint32]bool), ols: make(map[uint32]int), posAqRl: make(map[uint32]int)}
}

func newL() lock {
	return lock{rel: newvc2(), hb: newvc2(), count: 0, rels: make([]vcepoch, 0), acqRel: make([]acqRelPair, 0)} //rels: make(map[int]vcepoch, 0)}
}

type acqRelPair struct {
	owner uint32
	acq   epoch
	rel   vcepoch
	ols   map[uint32]int
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
	lwOls     map[uint32]int
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
	d1    *dot
	d2    *dot
	ols   map[uint32]int
	varid uint32
	debug uint32
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

	lock.acq = lock.nextAcq
	lock.hb = lock.hb.ssync(t1.vc)

	if len(t1.ls) > 0 { //remove lock from lockset
		t1.ls = t1.ls[:len(t1.ls)-1]
	}

	t1.vc = t1.vc.add(p.T1, 1)

	newAcqRelPair := acqRelPair{p.T1, newEpoch(lock.acq.t, lock.acq.v), lock.hb.clone(), make(map[uint32]int)}
	lock.ols = make(map[uint32]int)
	for _, k := range t1.ls { // open locks of thread
		l := locks[k]
		lock.ols[k] = l.count
		newAcqRelPair.ols[k] = l.count
	}

	lock.acqRel = append(lock.acqRel, newAcqRelPair)

	if lock.count >= len(lock.rels) { //no wrd add dummy vc
		lock.rels = append(lock.rels, newvc2())
	}

	locks[p.T2] = lock //doppelts zurueckschreiben noetig wegen racecheck!

	// newOpenRaces := make([]datarace, 0, len(openRaces))
	// for _, race := range openRaces {

	// 	if checkRace(race.ols, race.d1, race.d2, false, 0, race.varid) { //race complete!
	// 		//delete(openRaces, k)
	// 	} else {
	// 		newOpenRaces = append(newOpenRaces, race)
	// 	}

	// }
	// openRaces = newOpenRaces

	for _, v := range t1.lsboo {
		if v {
			lock.rel = lock.rel.ssync(t1.vc)
			break
		}
	}

	delete(t1.lsboo, p.T2)

	lock.count++

	threads[p.T1] = t1
	locks[p.T2] = lock
}

func (l *ListenerAsyncRcv) Putold(p *util.SyncPair) {
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

	lock.acq = lock.nextAcq
	lock.hb = lock.hb.ssync(t1.vc)
	if len(t1.ls) > 0 {
		t1.ls = t1.ls[:len(t1.ls)-1]
	}
	//delete(t1.ls, p.T2)

	t1.vc = t1.vc.add(p.T1, 1) //inc(Th(i),i)
	//t1.hb = t1.hb.add(p.T1, 1)

	lock.ols = make(map[uint32]int)
	for _, k := range t1.ls { // open locks of thread
		l := locks[k]
		lock.ols[k] = l.count
	}

	// for k := range t1.ols { //known open locks of t1 need to be transfered too!? ???????????????????????????????????????
	// 	lock.ols[k] = maxInt(lock.ols[k], t1.ols[k])
	// }

	// if _, ok := lock.rels[lock.count]; !ok { //no wrd add dummy vc
	// 	lock.rels[lock.count] = newvc2()
	// }

	if lock.count >= len(lock.rels) { //no wrd add dummy vc
		lock.rels = append(lock.rels, newvc2())
	}

	locks[p.T2] = lock //doppelts zurueckschreiben noetig wegen racecheck!

	//	fmt.Println("!!!!", p.T2, lock.rels)
	newOpenRaces := make([]datarace, 0, len(openRaces))
	for _, race := range openRaces {

		if checkRace(race.ols, race.d1, race.d2, false, 0, race.varid) { //race complete!
			//delete(openRaces, k)
		} else {
			newOpenRaces = append(newOpenRaces, race)
		}

	}
	openRaces = newOpenRaces

	if t1.strongSync == p.T2 {
		t1.strongSync = 0
	} else if t1.strongSync > 0 {
		lock.rel = lock.rel.ssync(t1.vc)
	}

	lock.count++

	threads[p.T1] = t1
	locks[p.T2] = lock
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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

	lock.nextAcq = newEpoch(p.T1, t1.vc.get(p.T1))

	t1.vc = t1.vc.ssync(lock.rel)
	t1.ls = append(t1.ls, p.T2)

	// for k, c := range lock.ols {  //entspricht happens-before die ols direkt zu uebertragen! machs erst entlang der WCP edges!
	// 	t1.ols[k] = maxInt(c, t1.ols[k])
	// }

	t1.vc = t1.vc.add(p.T1, 1)
	threads[p.T1] = t1
	locks[p.T2] = lock
}

func (l *ListenerAsyncSnd) Putold(p *util.SyncPair) {
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

	lock.nextAcq = newEpoch(p.T1, t1.vc.get(p.T1))

	t1.vc = t1.vc.ssync(lock.rel) //Th(i) = Th(i) U Rel(x)
	//t1.hb = t1.hb.ssync(lock.hb)  // Th(i)_hb = Th(i)_hb U Rel(x)_hb (hb synchro)
	t1.ls = append(t1.ls, p.T2)
	//t1.ls[p.T2] = struct{}{}

	t1.vc = t1.vc.add(p.T1, 1)
	//	t1.hb = t1.hb.add(p.T1, 1)

	for k, c := range lock.ols {
		t1.ols[k] = maxInt(c, t1.ols[k])
	}

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

func checkRace(ols map[uint32]int, prev *dot, curr *dot, addNew bool, level int, varid uint32) bool {

	currCompleteVC := curr.v.clone()

	for k, v := range ols {
		l := locks[k]

		ok := true
		if v >= len(l.rels) {
			ok = false
		}
		var vc vcepoch
		if ok {
			vc = newvc2()
			for i := 0; i < v; i++ {
				vc = vc.ssync(l.rels[i])
			}
			//vc = l.rels[v]
		}

		if ok { //vc exists
			currCompleteVC = currCompleteVC.ssync(vc)
		} else if addNew { //incomplete race, try it later again
			newDot := &dot{int: curr.int, ls: curr.ls, pos: curr.pos, sourceRef: curr.sourceRef, line: curr.line, t: curr.t, write: curr.write, v: curr.v.clone()}
			newRace := datarace{d1: prev, d2: newDot, ols: make(map[uint32]int), varid: 0, debug: varid}
			for k, v := range ols {
				newRace.ols[k] = v
			}
			openRaces = append(openRaces, newRace)

			return false
		} else {

			return false
		}
	}

	k := prev.v.get(uint32(prev.t))
	thi_at_j := currCompleteVC.get(uint32(prev.t))

	if k > thi_at_j {

		if !intersect(prev.ls, curr.ls) {

			report.ReportRace(report.Location{File: uint32(prev.sourceRef), Line: uint32(prev.line), W: prev.write},
				report.Location{File: uint32(curr.sourceRef), Line: uint32(curr.line), W: curr.write}, false, level)
		}
	}

	return true
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

	varstate.current++
	newFE := &dot{v: t1.vc.clone(), int: varstate.current, t: uint16(p.T1),
		sourceRef: p.Ev.Ops[0].SourceRef, line: uint16(p.Ev.Ops[0].Line),
		write: p.Write, ls: make(map[uint32]struct{}), pos: p.Ev.LocalIdx}
	for _, k := range t1.ls { //copy lockset
		newFE.ls[k] = struct{}{}
	}

	if p.Write {
		newFrontier := make([]*dot, 0, len(varstate.frontier))
		connectTo := make([]*dot, 0)

		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))
			thi_at_j := t1.vc.get(uint32(f.t))

			if k > thi_at_j {
				newFrontier = append(newFrontier, f) // RW(x) =  {j#k | j#k ∈ RW(x) ∧ k > Th(i)[j]}

				if !intersect(newFE.ls, f.ls) {
					checkRace(t1.ols, f, newFE, true, 0, p.T2)
					// report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
					// 	report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)
				}
				visited := &bitset.BitSet{}
				varstate.findRaces(t1.ols, newFE, f, visited, 0)

			} else if k < thi_at_j {
				connectTo = append(connectTo, f)
			}
		}

		varstate.updateGraph3(newFE, connectTo)

		newFrontier = append(newFrontier, newFE) // ∪{i#Th(i)[i]}
		varstate.frontier = newFrontier

		varstate.lastWrite = t1.vc.clone()
		varstate.lwLocks = make(map[uint32]struct{})
		for _, k := range t1.ls {
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
		newFE.v = newFE.v.ssync(varstate.lastWrite) //sync with last write in advance, necessary for the graph analysis in the following loop!

		newFrontier := make([]*dot, 0, len(varstate.frontier))
		connectTo := make([]*dot, 0)
		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))          //j#k
			thi_at_j := t1.vc.get(uint32(f.t)) //Th(i)[j]

			if k > thi_at_j {

				newFrontier = append(newFrontier, f) // RW(x) =  {j]k | j]k ∈ RW(x) ∧ k > Th(i)[j]}

				if f.write {
					if !intersect(newFE.ls, f.ls) {
						tmp := newFE.v
						newFE.v = t1.vc.clone()
						checkRace(t1.ols, f, newFE, true, 0, p.T2)
						newFE.v = tmp
						// report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
						// 	report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)
					}

					visited := &bitset.BitSet{}
					varstate.findRaces(t1.ols, newFE, f, visited, 0)
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

		//write-read sync
		t1.vc = t1.vc.ssync(varstate.lastWrite) //Th(i) = max(Th(i),L W (x))
		newFE.v = t1.vc.clone()

		varstate.updateGraph3(newFE, connectTo)

		newFrontier = append(newFrontier, newFE) // ∪{i#Th(i)[i]}
		varstate.frontier = newFrontier

		//connect to artifical start dot if no connection exists

		list, ok := varstate.graph.get(newFE.int)
		if ok && len(list) == 0 {
			list = append(list, &startDot)
			varstate.graph.add(newFE, list)
		}

		for _, k := range t1.ls {
			lk := locks[k]
			lk.rel = lk.rel.ssync(varstate.lastWrite) // collect lastwrite vcs in rel for each owned lock
			synced := false
			pos := 0
			//for i, p := range lk.acqRel {
			for i := t1.posAqRl[k]; i < len(lk.acqRel); i++ {
				p := lk.acqRel[i]
				localTimeForLastOwner := t1.vc.get(p.owner)
				relTimeForLastOwner := p.rel.get(p.owner)

				if p.acq.v < localTimeForLastOwner && localTimeForLastOwner < relTimeForLastOwner {
					synced = true
					t1.vc = t1.vc.ssync(p.rel)
					t1.lsboo[k] = true

					if i < len(lk.rels) {
						lk.rels[i] = p.rel
					} else {
						lk.rels = append(lk.rels, p.rel)
					}

					for k, c := range p.ols {
						t1.ols[k] = maxInt(c, t1.ols[k])
					}
				}

				if relTimeForLastOwner < localTimeForLastOwner {
					pos = i
				}

			}

			t1.posAqRl[k] = pos

			// lastOwner := lk.acq.t
			// localTimeForLastOwner := t1.vc.get(lastOwner)
			// relTimeForLastOwner := lk.hb.get(lastOwner)

			// if lk.acq.v < localTimeForLastOwner && localTimeForLastOwner < relTimeForLastOwner {
			// 	t1.vc = t1.vc.ssync(lk.hb)
			// 	lk.rels = append(lk.rels, lk.hb)
			// 	t1.lsboo[k] = true

			// 	for k, c := range lk.ols { //ols entlang wcp edge synchronisieren!
			// 		t1.ols[k] = maxInt(c, t1.ols[k])
			// 	}
			// 	// if t1.strongSync == 0 {
			// 	// 	t1.strongSync = t1.ls[0] //always the first is the most outer, might be to strong here and we can use the id of lk here?
			// 	// }
			// } else {
			if !synced {
				if _, ok := varstate.lwLocks[k]; ok {
					t1.vc = t1.vc.ssync(lk.hb)
					lk.rels = append(lk.rels, lk.hb)
					t1.lsboo[k] = true

					for k, c := range lk.ols { //ols entlang wcp edge synchronisieren!
						t1.ols[k] = maxInt(c, t1.ols[k])
					}
					// if t1.strongSync == 0 {
					// 	t1.strongSync = t1.ls[0] //always the first is the most outer, might be to strong here and we can use the id of lk here?
					// }
				}
			}
			//}

			locks[k] = lk
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

	t1.vc = t1.vc.add(p.T1, 1)
	threads[p.T1] = t1
	variables[p.T2] = varstate
}

func (l *ListenerDataAccess) Putold(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	// if p.Ev.Ops[0].SourceRef == 28 && p.Ev.Ops[0].Line == 114 && p.T2 == 379195 {
	// 	fmt.Println("first acc")
	// } else if p.Ev.Ops[0].SourceRef == 774 && p.Ev.Ops[0].Line == 66 && p.T2 == 379195 {
	// 	fmt.Println("second acc")
	// }

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
		for _, k := range t1.ls { //copy lockset
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
					checkRace(t1.ols, f, newFE, true, 0, p.T2)
					// report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
					// 	report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)
				}
				visited := &bitset.BitSet{}
				varstate.findRaces(t1.ols, newFE, f, visited, 0)

			} else if k < thi_at_j {
				connectTo = append(connectTo, f)
			}
		}

		varstate.updateGraph3(newFE, connectTo)

		newFrontier = append(newFrontier, newFE) // ∪{i#Th(i)[i]}
		varstate.frontier = newFrontier

		varstate.lastWrite = t1.vc.clone()
		varstate.lwLocks = make(map[uint32]struct{})
		for _, k := range t1.ls {
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
		for _, k := range t1.ls { //copy lockset
			newFE.ls[k] = struct{}{}
		}

		newFE.v = newFE.v.ssync(varstate.lastWrite) //sync with last write in advance, necessary for the graph analysis in the following loop!

		newFrontier := make([]*dot, 0, len(varstate.frontier))
		connectTo := make([]*dot, 0)
		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))          //j#k
			thi_at_j := t1.vc.get(uint32(f.t)) //Th(i)[j]

			if k > thi_at_j {

				newFrontier = append(newFrontier, f) // RW(x) =  {j]k | j]k ∈ RW(x) ∧ k > Th(i)[j]}

				if f.write {
					if !intersect(newFE.ls, f.ls) {
						tmp := newFE.v
						newFE.v = t1.vc.clone()
						checkRace(t1.ols, f, newFE, true, 0, p.T2)
						newFE.v = tmp
					}

					visited := &bitset.BitSet{}
					varstate.findRaces(t1.ols, newFE, f, visited, 0)
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

		//write-read sync
		t1.vc = t1.vc.ssync(varstate.lastWrite) //Th(i) = max(Th(i),L W (x))
		newFE.v = t1.vc.clone()

		varstate.updateGraph3(newFE, connectTo)

		newFrontier = append(newFrontier, newFE) // ∪{i#Th(i)[i]}
		varstate.frontier = newFrontier

		//connect to artifical start dot if no connection exists

		list, ok := varstate.graph.get(newFE.int)
		if ok && len(list) == 0 {
			list = append(list, &startDot)
			varstate.graph.add(newFE, list)
		}

		for _, k := range t1.ls {
			lk := locks[k]
			lk.rel = lk.rel.ssync(varstate.lastWrite)

			lastOwner := lk.acq.t
			localTimeForLastOwner := t1.vc.get(lastOwner)
			relTimeForLastOwner := lk.hb.get(lastOwner)

			if lk.acq.v < localTimeForLastOwner && localTimeForLastOwner < relTimeForLastOwner {
				t1.vc = t1.vc.ssync(lk.hb)
				lk.rels = append(lk.rels, lk.hb)

				if t1.strongSync == 0 {
					t1.strongSync = k //t1.ls[0] //always the first is the most outer, might be to strong here and we can use the id of lk here?
				}

			}
			locks[k] = lk
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
	//t1.hb = t1.hb.add(p.T1, 1)
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

	// if t1.strongSync > 0 {
	// 	signalList[p.T2] = t1.vc.clone().ssync(t1.hb)
	// }

	t1.vc = t1.vc.add(uint32(p.T1), 1)
	//t1.hb = t1.hb.add(p.T1, 1)
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
		//	t1.hb = t1.hb.add(p.T1, 1)
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
	// if t1.strongSync == 0 {
	// 	vc = vc.ssync(t1.vc)
	// } else {
	// 	vc = vc.ssync(t1.vc.clone().ssync(t1.hb))
	// }
	t1.vc = t1.vc.add(p.T1, 1)
	//	t1.hb = t1.hb.add(p.T1, 1)
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
		//t1.hb = t1.hb.add(p.T1, 1)
		vc = t1.vc.clone()
		// if t1.strongSync == 0 {
		// 	vc = t1.vc.clone()
		// } else {
		// 	vc = vc.ssync(t1.vc.clone().ssync(t1.hb))
		// }

		threads[p.T1] = t1
		notifies[p.T2] = vc
	}

}

func (l *ListenerPostProcess) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}
	//fmt.Println("!!!!", len(openRaces))
	checkOpenRaces()
	fmt.Println(">>>Max Open Races:", maxOpenRaces)
	// for _, race := range openRaces {
	// 	if checkRace(race.ols, race.d1, race.d2, false, 0, 0) { //race complete!
	// 		//delete(openRaces, k)
	// 	}
	// }
	//	fmt.Println("!!!!", len(openRaces))
}

func (v *variable) findRaces(ols map[uint32]int, raceAcc, prevAcc *dot, visited *bitset.BitSet, level uint64) {
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
				//report.ReportRace(report.Location{File: uint32(d.sourceRef), Line: uint32(d.line), W: d.write},
				//	report.Location{File: uint32(raceAcc.sourceRef), Line: uint32(raceAcc.line), W: raceAcc.write}, false, 1)
				checkRace(ols, d, raceAcc, true, 1, 0)
				// if b {
				// 	fmt.Println("LS's", raceAcc.ls, d.ls, intersect(raceAcc.ls, d.ls))
				// }
			}
			v.findRaces(ols, raceAcc, d, visited, level+1)
		}
	}

}
