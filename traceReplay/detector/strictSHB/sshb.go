package sshb

import (
	"fmt"
	"os"

	"../../parser"
	"../../util"
	algos "../analysis"
	"../report"
	"../traceReplay"
)

type ListenerAsyncSnd struct{}
type ListenerAsyncRcv struct{}
type ListenerSync struct{}
type ListenerDataAccess struct{}
type ListenerDataAccessALL struct{}
type ListenerDataAccessHB struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}
type ListenerNT struct{}
type ListenerNTWT struct{}
type ListenerPostProcess struct{}
type ListenerPostProcessALL struct{}
type ListenerPostProcessPara struct{}
type ListenerPostProcessLarge struct{}

type EventCollector struct {
	listeners []traceReplay.EventListener
}

var statistics = true
var doPostProcess = true

func Init() {
	threads = make(map[uint32]thread)
	locks = make(map[uint32]lock)
	signalList = make(map[uint32]signal)
	variables = make(map[uint32]variable)

	volatiles = make(map[uint32]vcepoch)
	notifies = make(map[uint32]vcepoch)

	listeners1 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccess{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("sshb", &EventCollector{listeners1})

	listenersALL := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessALL{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcessALL{},
	}
	algos.RegisterDetector("sshbALL", &EventCollector{listenersALL})

	listenersALLPara := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessALL{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcessPara{},
	}
	algos.RegisterDetector("sshbALLPARA", &EventCollector{listenersALLPara})

	listeners2 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccess{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcessLarge{},
	}
	algos.RegisterDetector("sshbLarge", &EventCollector{listeners2})
}

var threads map[uint32]thread
var locks map[uint32]lock
var signalList map[uint32]signal
var variables map[uint32]variable

var volatiles map[uint32]vcepoch
var notifies map[uint32]vcepoch

type thread struct {
	ls    map[uint32]struct{}
	vc    vcepoch
	lw    vcepoch
	curr  *node
	first *node
}

func newThread(tid uint32) thread {
	return thread{vc: newvc2().set(tid, 1), curr: nil, first: nil, ls: make(map[uint32]struct{}), lw: newvc2().set(tid, 1)}
}

var threads2 = make(map[uint32][]*node)

type lock struct {
	vc   vcepoch
	curr *node
}

type signal struct {
	vc   vcepoch
	curr *node
}

type variable struct {
	writes        []*node
	reads         []*node
	races         []race
	lastWrite     vcepoch
	rvc           vcepoch
	wvc           vcepoch
	lwEv          *util.Item
	lastEv        *util.Item
	lastWriteNode *node
	hasRace       bool
}

var nodes []*node

type race struct {
	acc1    *node
	acc2    *node
	lsEmpty bool
}

type variableHistory struct {
	ev    *util.Item
	clock vcepoch
}

type node struct {
	ls       map[uint32]struct{}
	next     []*node
	clock    vcepoch
	clock2   vcepoch
	ev       *util.Item
	inWrites uint
	visited  bool
}

func newVar() variable {
	return variable{lastWrite: newvc2(), lwEv: nil, rvc: newEpoch(0, 0), wvc: newEpoch(0, 0),
		lastEv: nil, hasRace: false, writes: make([]*node, 0), reads: make([]*node, 0), races: make([]race, 0), lastWriteNode: nil}
}

func (l *EventCollector) Put(p *util.SyncPair) {
	//	syncPairTrace = append(syncPairTrace, p)
	for _, l := range l.listeners {
		l.Put(p)
	}
}

var uniqueRaceFilter = make(map[string]map[string]struct{})

func isUnique(r race) bool {
	s1 := fmt.Sprintf("f:%vl%v", r.acc1.ev.Ops[0].SourceRef, r.acc1.ev.Ops[0].Line)
	s2 := fmt.Sprintf("f:%vl%v", r.acc2.ev.Ops[0].SourceRef, r.acc2.ev.Ops[0].Line)

	s1map := uniqueRaceFilter[s1]
	if s1map == nil {
		s1map = make(map[string]struct{})
	}

	if _, ok := s1map[s2]; !ok {
		s1map[s2] = struct{}{}
		s2map := uniqueRaceFilter[s2]
		if s2map == nil {
			s2map = make(map[string]struct{})
		}
		s2map[s1] = struct{}{}
		uniqueRaceFilter[s1] = s1map
		uniqueRaceFilter[s2] = s2map
		return true
	}
	return false
}

func intersect(a, b map[uint32]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; ok {
			return true
		}
	}
	return false
}

var readLocMap = make(map[string]uint)

var invalidRaces = 0
var validRaces = 0
var defWrdEnough = 0
var lsFP = 0

func (l *ListenerDataAccess) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread(p.T1)
	}

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar()
	}

	newNode := &node{ev: p.Ev, clock: t1.vc.clone().trim(), next: make([]*node, 0), ls: make(map[uint32]struct{})}
	for k := range t1.ls {
		newNode.ls[k] = struct{}{}
	}
	//fmt.Println("LS new node", newNode.ls)
	//fmt.Println("LS thread", t1.ls)
	//Program order
	if t1.curr != nil {
		t1.curr.next = append(t1.curr.next, newNode)
	}
	//nodes = append(nodes, newNode)
	t1.curr = newNode

	if p.Write {
		// list := threads2[p.T1]
		// list = append(list, newNode)
		// threads2[p.T1] = list

		newWrites := make([]*node, 0)
		//if !varstate.wvc.leq(t1.vc) {
		//concurrent writes exist
		for i, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				newWrites = append(newWrites, varstate.writes[i])
				//fmt.Println("Intersect(", t1.ls, w.ls, ")", intersect(t1.ls, w.ls))
				r := race{varstate.writes[i], newNode, !intersect(t1.ls, w.ls)}
				//if !intersect(t1.ls, w.ls) {
				//if !intersect(w.ls, newNode.ls) {
				l1 := report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0}
				l2 := report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0}
				b := report.ReportRace(l1, l2, false, 0)
				if b {
					r.acc2.clock2 = t1.vc.clone().ssync(t1.lw).trim()
					varstate.races = append(varstate.races, r)
					// if !checkRaceValidity(r) {
					// 	fmt.Println("ValidRace")
					// 	validRaces++
					// 	if intersect(w.ls, newNode.ls) {
					// 		lsFP++
					// 	}
					// 	//report.ReportRace(l1, l2, false, 0)
					// } else {
					// 	fmt.Println("InvalidRace")
					// 	invalidRaces++

					// 	k := w.clock.get(w.ev.Thread)
					// 	curr := t1.vc.clone().ssync(t1.lw).get(w.ev.Thread)
					// 	if k <= curr {
					// 		defWrdEnough++
					// 	}
					// }
				}
				//}
				//varstate.races = append(varstate.races, r)
				//}
			} else {
				for _, n := range w.next {
					if n.ev.Ops[0].Kind&util.READ > 0 {
						n.inWrites++
						parser.Parserlock.Lock()
						f1 := parser.FileNumToString[n.ev.Ops[0].SourceRef]
						parser.Parserlock.Unlock()
						//s := fmt.Sprintf("%v:%v", n.ev.Ops[0].SourceRef, n.ev.Ops[0].Line)
						s := fmt.Sprintf("%v:%v", f1, n.ev.Ops[0].Line)
						c := readLocMap[s]
						if n.inWrites > c {
							readLocMap[s] = n.inWrites
						}
						// val := readCounts[n]
						// val++
						// readCounts[n] = val
					}
				}
			}
		}
		//}
		newWrites = append(newWrites, newNode)
		varstate.wvc = varstate.wvc.set(p.T1, t1.vc.get(p.T1))

		//	if !varstate.rvc.leq(t1.vc) {
		//concurrent reads exist
		for i, r := range varstate.reads {
			k := r.clock.get(r.ev.Thread)
			curr := t1.vc.get(r.ev.Thread)
			if k > curr {
				//update wrd graph
				newNode.next = append(newNode.next, varstate.reads[i])
				//store race
				q := race{varstate.reads[i], newNode, !intersect(t1.ls, r.ls)}

				//	if !intersect(t1.ls, r.ls) {
				//if !intersect(r.ls, newNode.ls) {
				l1 := report.Location{File: q.acc1.ev.Ops[0].SourceRef, Line: q.acc1.ev.Ops[0].Line, W: q.acc1.ev.Ops[0].Kind&util.WRITE > 0}
				l2 := report.Location{File: q.acc2.ev.Ops[0].SourceRef, Line: q.acc2.ev.Ops[0].Line, W: q.acc2.ev.Ops[0].Kind&util.WRITE > 0}
				b := report.ReportRace(l1, l2, false, 0)
				if b {
					q.acc2.clock2 = t1.vc.clone().ssync(t1.lw)
					varstate.races = append(varstate.races, q)
					// if !checkRaceValidity(q) {
					// 	fmt.Println("ValidRace")
					// 	validRaces++
					// 	//report.ReportRace(l1, l2, false, 0)
					// 	if intersect(r.ls, newNode.ls) {
					// 		lsFP++
					// 	}
					// } else {
					// 	fmt.Println("InvalidRace")
					// 	invalidRaces++

					// 	k := r.clock.get(r.ev.Thread)
					// 	curr := t1.vc.clone().ssync(t1.lw).get(r.ev.Thread)
					// 	if k <= curr {
					// 		defWrdEnough++
					// 	}
					// }
				}
				//	}
				//varstate.races = append(varstate.races, q)
				//}
			}
		}
		//	}
		varstate.lastWrite = t1.vc.clone()
		varstate.writes = newWrites
		t1.vc = t1.vc.add(p.T1, 1)
	} else if p.Read {
		//find concurrent writes, add wrd and store the detected race
		//if !varstate.wvc.leq(t1.vc) {
		for i, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				varstate.writes[i].next = append(varstate.writes[i].next, newNode)
				r := race{varstate.writes[i], newNode, !intersect(t1.ls, w.ls)}

				//	if !intersect(t1.ls, w.ls) {
				//if !intersect(w.ls, newNode.ls) {
				l1 := report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0}
				l2 := report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0}
				b := report.ReportRace(l1, l2, false, 0)
				if b {
					r.acc2.clock2 = t1.vc.clone().ssync(t1.lw)
					varstate.races = append(varstate.races, r)
					// if !checkRaceValidity(r) {
					// 	fmt.Println("ValidRace")
					// 	validRaces++
					// 	if intersect(w.ls, newNode.ls) {
					// 		lsFP++
					// 	}
					// 	//report.ReportRace(l1, l2, false, 0)
					// } else {
					// 	fmt.Println("InvalidRace")
					// 	invalidRaces++

					// 	k := w.clock.get(w.ev.Thread)
					// 	curr := t1.vc.clone().ssync(t1.lw).get(w.ev.Thread)
					// 	if k <= curr {
					// 		defWrdEnough++
					// 	}
					// }
				}
				// if !checkRaceValidity(r) {
				// 	report.RaceStatistics2(
				// 		report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
				// 		report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
				// 		false, 0)
				// }
				//varstate.races = append(varstate.races, r)
				//}
				//	}
			}
		}
		//	}
		//t1.vc = t1.vc.ssync(varstate.lastWrite)
		t1.lw = t1.lw.ssync(varstate.lastWrite)
		newReads := make([]*node, 0)
		//if !varstate.rvc.leq(t1.vc) {
		for i, r := range varstate.reads {
			k := r.clock.get(r.ev.Thread)
			curr := t1.vc.get(r.ev.Thread)
			if k > curr {
				newReads = append(newReads, varstate.reads[i])
			}
		}
		//}
		//t1.vc = t1.vc.ssync(varstate.lastWrite)
		//newNode.clock = t1.vc.clone()
		newReads = append(newReads, newNode)

		varstate.rvc = varstate.rvc.set(p.T1, t1.vc.get(p.T1))
		varstate.reads = newReads
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

func (l *ListenerDataAccessALL) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread(p.T1)
	}

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar()
	}

	newNode := &node{ev: p.Ev, clock: t1.vc.clone().trim(), clock2: t1.vc.clone().ssync(t1.lw).trim(), next: make([]*node, 0), ls: make(map[uint32]struct{})}
	for k := range t1.ls {
		newNode.ls[k] = struct{}{}
	}

	//Program order
	if t1.curr != nil {
		t1.curr.next = append(t1.curr.next, newNode)
	}
	//nodes = append(nodes, newNode)
	t1.curr = newNode

	if p.Write {
		newWrites := make([]*node, 0)
		//concurrent writes exist
		for i, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				newWrites = append(newWrites, varstate.writes[i])
				r := race{varstate.writes[i], newNode, false}

				//l1 := report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0}
				//l2 := report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0}
				//b := report.ReportRace(l1, l2, false, 0)
				//if b {
				//	r.acc2.clock2 = t1.vc.clone().ssync(t1.lw)
				if !intersect(w.ls, newNode.ls) {
					varstate.races = append(varstate.races, r)
				}
				//	}

			}
		}
		//}
		newWrites = append(newWrites, newNode)
		varstate.wvc = varstate.wvc.set(p.T1, t1.vc.get(p.T1))

		//	if !varstate.rvc.leq(t1.vc) {
		//concurrent reads exist
		for i, r := range varstate.reads {
			k := r.clock.get(r.ev.Thread)
			curr := t1.vc.get(r.ev.Thread)
			if k > curr {
				//update wrd graph
				newNode.next = append(newNode.next, varstate.reads[i])
				//store race
				q := race{varstate.reads[i], newNode, false}
				//if !intersect(r.ls, newNode.ls) {
				// l1 := report.Location{File: q.acc1.ev.Ops[0].SourceRef, Line: q.acc1.ev.Ops[0].Line, W: q.acc1.ev.Ops[0].Kind&util.WRITE > 0}
				// l2 := report.Location{File: q.acc2.ev.Ops[0].SourceRef, Line: q.acc2.ev.Ops[0].Line, W: q.acc2.ev.Ops[0].Kind&util.WRITE > 0}
				// b := report.ReportRace(l1, l2, false, 0)
				// if b {
				// 	q.acc2.clock2 = t1.vc.clone().ssync(t1.lw)
				// 	varstate.races = append(varstate.races, q)
				// }

				if !intersect(r.ls, newNode.ls) {
					varstate.races = append(varstate.races, q)
				}
			}
		}

		varstate.lastWrite = t1.vc.clone()
		varstate.writes = newWrites
		t1.vc = t1.vc.add(p.T1, 1)
	} else if p.Read {
		//find concurrent writes, add wrd and store the detected race
		for i, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				varstate.writes[i].next = append(varstate.writes[i].next, newNode)
				r := race{varstate.writes[i], newNode, false}
				// l1 := report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0}
				// l2 := report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0}
				// b := report.ReportRace(l1, l2, false, 0)
				// if b {
				// 	r.acc2.clock2 = t1.vc.clone().ssync(t1.lw)
				// 	varstate.races = append(varstate.races, r)
				// }

				if !intersect(w.ls, newNode.ls) {
					varstate.races = append(varstate.races, r)
				}

			}
		}

		t1.lw = t1.lw.ssync(varstate.lastWrite)
		newReads := make([]*node, 0)

		for i, r := range varstate.reads {
			k := r.clock.get(r.ev.Thread)
			curr := t1.vc.get(r.ev.Thread)
			if k > curr {
				newReads = append(newReads, varstate.reads[i])
			}
		}

		newReads = append(newReads, newNode)

		varstate.rvc = varstate.rvc.set(p.T1, t1.vc.get(p.T1))
		varstate.reads = newReads
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

func (l *ListenerAsyncSnd) Put(p *util.SyncPair) {
	if !p.AsyncSend {
		return
	}

	if !p.Lock {
		return
	}

	lock, ok := locks[p.T2]
	if !ok {
		lock.vc = newvc2()
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread(p.T1)
	}

	t1.vc = t1.vc.ssync(lock.vc)

	//Program order
	// newNode := &node{ev: p.Ev, clock: t1.vc.clone(), next: make([]*node, 0)}
	// if t1.curr != nil { //first node
	// 	t1.curr.next = append(t1.curr.next, newNode)
	// }
	// t1.curr = newNode
	//nodes = append(nodes, newNode)
	// list := threads2[p.T1]
	// list = append(list, newNode)
	// threads2[p.T1] = list

	//RAD order
	// if lock.curr != nil {
	// 	lock.curr.next = append(lock.curr.next, newNode)
	// }

	t1.ls[p.T2] = struct{}{}
	t1.vc = t1.vc.add(p.T1, 1)

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
		lock.vc = newvc2()
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread(p.T1)
	}

	//Program order
	// newNode := &node{ev: p.Ev, clock: t1.vc.clone(), next: make([]*node, 0)}
	// if t1.curr != nil { //first node
	// 	t1.curr.next = append(t1.curr.next, newNode)
	// }
	// t1.curr = newNode
	//nodes = append(nodes, newNode)
	// list := threads2[p.T1]
	// list = append(list, newNode)
	// threads2[p.T1] = list

	//RAD order
	//lock.curr = newNode

	lock.vc = t1.vc.clone()
	t1.vc = t1.vc.add(p.T1, 1)

	delete(t1.ls, p.T2)

	threads[p.T1] = t1
	locks[p.T2] = lock
}

func (l *ListenerSync) Put(p *util.SyncPair) {
	if !p.Sync {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread(p.T1)
	}
	t2, ok := threads[p.T2]
	if !ok {
		t2 = newThread(p.T2)
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
	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread(p.T1)
	}

	//Program order
	newNode := &node{ev: p.Ev, clock: t1.vc.clone(), next: make([]*node, 0)}
	if t1.curr != nil { //first node
		t1.curr.next = append(t1.curr.next, newNode)
	}
	t1.curr = newNode
	//nodes = append(nodes, newNode)
	// list := threads2[p.T1]
	// list = append(list, newNode)
	// threads2[p.T1] = list

	signalList[p.T2] = signal{t1.vc.clone(), newNode}

	t1.vc = t1.vc.add(p.T1, 1)

	threads[p.T1] = t1
}

func (l *ListenerGoWait) Put(p *util.SyncPair) {
	if !p.IsWait {
		return
	}

	vc, ok := signalList[p.T2]

	if ok {
		t1, ok := threads[p.T1]
		if !ok {
			t1 = newThread(p.T1)
		}
		t1.vc = t1.vc.ssync(vc.vc)
		t1.vc = t1.vc.add(p.T1, 1)

		//Program order
		newNode := &node{ev: p.Ev, clock: t1.vc.clone(), next: make([]*node, 0)}
		if t1.curr != nil { //first node
			t1.curr.next = append(t1.curr.next, newNode)
		}
		t1.curr = newNode
		//nodes = append(nodes, newNode)
		// list := threads2[p.T1]
		// list = append(list, newNode)
		// threads2[p.T1] = list

		//Fork order
		vc.curr.next = append(vc.curr.next, newNode)

		threads[p.T1] = t1
	}
}
func (l *ListenerNT) Put(p *util.SyncPair) {
	if !p.IsNT {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread(p.T1)
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
			t1 = newThread(p.T1)
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

	fmt.Println("Post processing starts...")
	sumRaces := 0
	for _, v := range variables {
		sumRaces += len(v.races)
	}

	wwrace := 0
	wrrace := 0
	rwrace := 0

	//falsePositives := 0

	steps := 0
	complete := sumRaces
	stepBarrier := (complete / 20) + 1

	for _, v := range variables {
		for _, r := range v.races {
			if !checkRaceValidity(r) {
				//fmt.Println("ValidRace")
				validRaces++
				if intersect(r.acc1.ls, r.acc2.ls) {
					lsFP++
				} else {

				}
				if r.acc1.ev.Ops[0].Kind&util.WRITE > 0 {
					if r.acc2.ev.Ops[0].Kind&util.WRITE > 0 {
						wwrace++
					} else {
						wrrace++
					}
				} else {
					rwrace++
				}
				//report.ReportRace(l1, l2, false, 0)
			} else {
				//fmt.Println("InvalidRace")
				invalidRaces++

				k := r.acc1.clock.get(r.acc1.ev.Thread)
				curr := r.acc2.clock2.get(r.acc1.ev.Thread)
				if k <= curr {
					defWrdEnough++
				}
			}
			// // check the race if its valide
			// if !checkRaceValidity(r) {
			// 	report.RaceStatistics2(
			// 		report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
			// 		report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
			// 		false, 0)
			// 	if !r.lsEmpty {
			// 		falsePositives++
			// 	}
			// }
			steps++
			if steps%stepBarrier == 0 {
				fmt.Printf("\r%v/%v", steps, complete)
			}
		}
	}

	fmt.Printf("Variables:%v\nDynamicRaces:%v\nUniqueRaces:%v\n", len(variables), sumRaces, sumRaces)
	fmt.Println("VALID RACES:", validRaces)
	fmt.Println("INVALID RACES:", invalidRaces)
	fmt.Println("DEF WRD ENOUGH:", defWrdEnough)
	fmt.Println("FALSE POSITIVES:", lsFP)

	fmt.Println("WWRace:", wwrace)
	fmt.Println("WRRace:", wrrace)
	fmt.Println("RWRace:", rwrace)

	//	countAlternativeWrites()

	report.ReportFinish()
}

func (l *ListenerPostProcessALL) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}
	threads = nil
	locks = nil
	//variables = nil
	notifies = nil

	fmt.Println("Post processing starts...")
	sumRaces := 0
	for _, v := range variables {
		sumRaces += len(v.races)
	}
	fmt.Println("SUMRACES:", sumRaces)

	raceAccRej := make(map[string][2]uint)

	//falsePositives := 0

	steps := 0
	complete := sumRaces
	stepBarrier := (complete / 20) + 1

	for _, v := range variables {
		for _, r := range v.races {
			strPair := fmt.Sprintf("%v:%v,%v:%v", r.acc1.ev.Ops[0].SourceRef, r.acc1.ev.Ops[0].Line, r.acc2.ev.Ops[0].SourceRef, r.acc2.ev.Ops[0].Line)
			res := raceAccRej[strPair]

			k := r.acc1.clock2.get(r.acc1.ev.Thread)
			curr := r.acc2.clock2.get(r.acc1.ev.Thread)
			if k > curr {
				if !checkRaceValidity(r) {
					validRaces++
					res[0]++
				} else {
					invalidRaces++
					res[1]++
				}
			} else {
				invalidRaces++
				res[1]++
				defWrdEnough++
			}

			raceAccRej[strPair] = res

			steps++
			if steps%stepBarrier == 0 {
				fmt.Printf("\r%v/%v", steps, complete)
			}
		}
	}
	fmt.Println("RACES:", len(raceAccRej))
	fullI := 0
	fullG := 0
	moreG := 0
	neutral := 0
	moreI := 0
	for k, v := range raceAccRej {
		fmt.Printf("Pair:%v\nG:%v\nI:%v\n", k, v[0], v[1])
		if v[0] == 0 {
			fullI++
		} else if v[1] == 0 {
			fullG++
		} else if v[0] > v[1] {
			moreG++
		} else if v[0] == v[1] {
			neutral++
		} else {
			moreI++
		}
	}

	fmt.Printf("FullG:%v\nFullI:%v\nMoreG:%v\nMoreI:%v\nNeutral:%v\n", fullG, fullI, moreG, moreI, neutral)
	fmt.Printf("Standard WRD ENOUGH:%v/%v\n", defWrdEnough, sumRaces)
	report.ReportFinish()
}

type raceValid struct {
	s         string
	valid     bool
	defEnough bool
}

func (l *ListenerPostProcessPara) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}
	threads = nil
	locks = nil
	//variables = nil
	notifies = nil

	fmt.Println("Post processing starts...")
	sumRaces := 0
	for _, v := range variables {
		sumRaces += len(v.races)
	}
	fmt.Println("SUMRACES:", sumRaces)

	raceAccRej := make(map[string][2]uint)

	input := make(chan race, 1000)
	output := make(chan raceValid)

	go func() {
		for _, v := range variables {
			for _, r := range v.races {
				input <- r
			}
			v.races = nil
		}
	}()

	for i := 0; i < 60; i++ {
		go func() {
			for {
				r, ok := <-input
				if !ok {
					return
				}
				strPair := fmt.Sprintf("%v:%v,%v:%v", r.acc1.ev.Ops[0].SourceRef, r.acc1.ev.Ops[0].Line, r.acc2.ev.Ops[0].SourceRef, r.acc2.ev.Ops[0].Line)
				k := r.acc1.clock2.get(r.acc1.ev.Thread)
				curr := r.acc2.clock2.get(r.acc1.ev.Thread)
				if k > curr {
					if !checkRaceValidity(r) {
						output <- raceValid{s: strPair, valid: true}
					} else {
						output <- raceValid{s: strPair, valid: false}
					}
				} else {
					output <- raceValid{s: strPair, valid: false, defEnough: true}
				}
			}
		}()
	}

	steps := 0
	complete := sumRaces
	stepBarrier := (complete / 40) + 1

	for i := 0; i < sumRaces; i++ {
		r := <-output
		res := raceAccRej[r.s]

		if r.valid {
			validRaces++
			res[0]++
		} else {
			invalidRaces++
			res[1]++
		}

		if r.defEnough {
			defWrdEnough++
		}

		raceAccRej[r.s] = res
		steps++
		if steps%stepBarrier == 0 {
			fmt.Printf("\r%v/%v", steps, complete)
		}
	}

	fmt.Println("RACES:", len(raceAccRej))
	fullI := 0
	fullG := 0
	moreG := 0
	neutral := 0
	moreI := 0
	for k, v := range raceAccRej {
		fmt.Printf("Pair:%v\nG:%v\nI:%v\n", k, v[0], v[1])
		if v[0] == 0 {
			fullI++
		} else if v[1] == 0 {
			fullG++
		} else if v[0] > v[1] {
			moreG++
		} else if v[0] == v[1] {
			neutral++
		} else {
			moreI++
		}
	}

	fmt.Printf("FullG:%v\nFullI:%v\nMoreG:%v\nMoreI:%v\nNeutral:%v\n", fullG, fullI, moreG, moreI, neutral)
	fmt.Printf("Standard WRD ENOUGH:%v/%v\n", defWrdEnough, sumRaces)
	report.ReportFinish()
}

func (l *ListenerPostProcessLarge) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}

	fmt.Println("Post processing starts...")
	sumRaces := 0
	for _, v := range variables {
		sumRaces += len(v.races)
	}
	fmt.Printf("Variables:%v\nDynamicRaces:%v\nUniqueRaces:%v\n", len(variables), sumRaces, sumRaces)

	steps := 0
	complete := sumRaces
	stepBarrier := (complete / 50) + 1

	for _, v := range variables {
		for _, r := range v.races {
			// check the race if its valide
			if !checkRaceValidityLarge(r) {
				report.RaceStatistics2(
					report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
					report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
					false, 0)
			}
			steps++
			if steps%stepBarrier == 0 {
				fmt.Printf("\r%v/%v", steps, complete)
			}
		}
	}
	fmt.Println("Sum Races", sumRaces)
}

func checkRaceValidity(r race) bool {
	valid, _ := search(r.acc1, r.acc2, make(map[*node]struct{}), 0)
	for i := range nodes {
		nodes[i].visited = false
	}

	return valid
}
func checkRaceValidityLarge(r race) bool {
	valid, _ := dsearch(r.acc1, r.acc2, make(map[*node]struct{}), 0)
	for i := range nodes {
		nodes[i].visited = false
	}

	return valid
}

func trivialPathExists(r race) bool {
	for _, c := range r.acc1.next {
		if c == r.acc2 {
			return true
		}
	}
	return false
}

var reachMap = make(map[*node][]*node)

func search(current, finish *node, visited map[*node]struct{}, level int) (bool, int) {

	visited[current] = struct{}{}

	// if level > 200 {
	// 	return false, level
	// 	return dsearch(current, finish, visited, level)
	// }
	if level > 10000 {
		//return false, level
		return dsearch(current, finish, visited, level)
	}

	//current.visited = true

	k := finish.clock.get(finish.ev.Thread)
	curr := current.clock.get(finish.ev.Thread)
	if k < curr {
		return false, level
	}

	if current == finish { //found a way
		return true, level
	}

	if len(current.next) == 0 { //no child nodes
		return false, level
	}

	//breadth part
	for i := range current.next {
		if level != 0 && current.next[i] == finish {
			return true, level
		}
	}

	for i := range current.next {
		//if _, f := visited[current.next[i]]; !f {
		//if !current.next[i].visited {
		if _, ok := visited[current.next[i]]; !ok {
			if !(level == 0 && current.next[i] == finish) {
				if ok, el := search(current.next[i], finish, visited, level+1); ok { //external links
					// if level == 0 {
					// 	fmt.Println("!!!!!!", el)
					// }
					return true, el // append(path, p...)
				}
			}
		}
		//}
	}

	return false, level
}

func bsearch(start, finish *node, level int) (bool, int) {
	if start == finish {
		return true, 0
	}

	queue := make([]*node, 0)
	queue = append(queue, start)

	var current *node

	for len(queue) != 0 {
		current, queue = queue[0], queue[1:]
		if current == finish {
			return true, 0
		}
		if len(current.next) == 0 {
			return false, 0
		}
		queue = append(queue, current.next...)
	}

	return false, 0
}

func dsearch(start, finish *node, visited map[*node]struct{}, level int) (bool, int) {
	stack := make([]*node, 0, 5000)
	stack = append(stack, start)
	var curr *node

	k := finish.clock.get(finish.ev.Thread)

	for len(stack) != 0 {
		curr, stack = stack[len(stack)-1], stack[:len(stack)-1]

		visited[curr] = struct{}{}
		// if !curr.visited {
		// 	curr.visited = true
		// }

		if curr == finish {
			return true, 0
		}

		for i := range curr.next {
			if _, ok := visited[curr.next[i]]; !ok && !(level == 0 && curr.next[i] == finish) {
				//if !curr.next[i].visited && !(level == 0 && curr.next[i] == finish) { //level==0 filters the trivial path
				nextClock := curr.next[i].clock.get(finish.ev.Thread)
				if !(k < nextClock) {
					stack = append(stack, curr.next[i])
				}
			}
		}
		level++
	}

	return false, 0
}

var readCounts = make(map[*node]uint)

func countAlternativeWrites() {

	// for _, t := range threads2 {
	// 	for _, x := range t {
	// 		if x.ev.Ops[0].Kind&util.WRITE > 0 {
	// 			for _, n := range x.next {
	// 				if n.ev.Ops[0].Kind&util.READ > 0 {
	// 					val := readCounts[n]
	// 					val++
	// 					readCounts[n] = val
	// 				}
	// 			}
	// 		}
	// 	}
	// }

	for _, v := range variables {
		for _, w := range v.writes {
			for _, n := range w.next {
				if n.ev.Ops[0].Kind&util.READ > 0 {
					n.inWrites++
					f1 := parser.FileNumToString[n.ev.Ops[0].SourceRef]
					//s := fmt.Sprintf("%v:%v", n.ev.Ops[0].SourceRef, n.ev.Ops[0].Line)
					s := fmt.Sprintf("%v:%v", f1, n.ev.Ops[0].Line)
					c := readLocMap[s]
					if n.inWrites > c {
						readLocMap[s] = n.inWrites
					}
					val := readCounts[n]
					val++
					readCounts[n] = val
				}
			}
			// for _, n := range w.next {
			// 	if n.ev.Ops[0].Kind&util.READ > 0 {
			// 		val := readCounts[n]
			// 		val++
			// 		readCounts[n] = val
			// 	}
			// }
		}
	}

	sum := uint(0)
	max := uint(0)
	notOne := 0
	perVariable := make(map[uint32][2]uint)
	for n, v := range readCounts {
		sum += v
		if v > max {
			max = v
		}
		count := perVariable[n.ev.Ops[0].Ch]
		count[0] += v
		count[1]++
		perVariable[n.ev.Ops[0].Ch] = count
		if v != 1 {
			notOne++
		}
	}
	fmt.Printf("Max writes:%v\nAvg writes:%v\nNotOne:%v\n", max, float64(sum)/float64(len(readCounts)), notOne)

	f, err := os.Create("distResults.txt")
	if err != nil {
		panic(err)
	}
	for k, v := range perVariable {
		f.WriteString(fmt.Sprintf("%v,%v,%v\n", k, v[0], v[1]))
	}
	f.Sync()
	f.Close()

	f2, err := os.Create("readLocCount.txt")
	if err != nil {
		panic(err)
	}
	for k, v := range readLocMap {
		f2.WriteString(fmt.Sprintf("%v,%v\n", k, v))
	}
}
