package shbp

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
type ListenerDataAccessSHB struct{}
type ListenerDataAccessHB struct{}
type ListenerDataAccessSHBNOLS struct{}
type ListenerDataAccessHBNOLS struct{}
type ListenerDataAccessHBNOP struct{}
type ListenerDataAccessSHBSingle struct{}
type ListenerDataAccessSHBWrites struct{}
type ListenerDataAccessSHBDefault struct{}
type ListenerDataAccessSHBNoPartner struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}
type ListenerNT struct{}
type ListenerNTWT struct{}
type ListenerPostProcess struct{}

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
		&ListenerDataAccessSHB{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("shbp", &EventCollector{listeners1})

	algos.RegisterDetector("shbpNOLS", &EventCollector{[]traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessSHBNOLS{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}})

	//algos.RegisterDetector("mfshbee", &EventCollector{listeners1})

	listeners2 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessHB{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("hbp", &EventCollector{listeners2})

	algos.RegisterDetector("hbpNOLS", &EventCollector{[]traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessHBNOLS{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}})

	listenershbdef := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessHBNOP{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("hbDefault", &EventCollector{listenershbdef})

	listeners3 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessSHBSingle{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("shbpSingle", &EventCollector{listeners3})

	listeners4 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessSHBWrites{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("shbpWrites", &EventCollector{listeners4})

	listeners5 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessSHBNoPartner{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("shbDefault", &EventCollector{listeners5})

}

var threads map[uint32]thread
var locks map[uint32]lock
var signalList map[uint32]signal
var variables map[uint32]variable
var volatiles map[uint32]vcepoch
var notifies map[uint32]vcepoch

type thread struct {
	vc   vcepoch
	curr *node
	ls   map[uint32]struct{}
}

func newThread(tid uint32) thread {
	return thread{newvc2().set(tid, 1), nil, make(map[uint32]struct{})}
}

type lock struct {
	vc   vcepoch
	curr *node
}

type signal struct {
	vc vcepoch
}

type variable struct {
	lastWrite vcepoch
	lwEv      *util.Item
	rvc       vcepoch
	wvc       vcepoch
	lastEv    *util.Item
	hasRace   bool
	writes    []*node
	reads     []*node
	races     []race
	lwNode    *node
}

var nodes []*node

type race struct {
	acc1 *node
	acc2 *node
}

type variableHistory struct {
	ev    *util.Item
	clock vcepoch
}

type node struct {
	ls    map[uint32]struct{}
	ev    *util.Item
	clock vcepoch
}

func newVar() variable {
	return variable{newvc2(), nil, newEpoch(0, 0), newEpoch(0, 0),
		nil, false, make([]*node, 0), make([]*node, 0), make([]race, 0), nil}
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

func (l *ListenerDataAccessHB) Put(p *util.SyncPair) {
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
	newNode := &node{ev: p.Ev, clock: t1.vc.clone(), ls: make(map[uint32]struct{})}
	for k := range t1.ls {
		newNode.ls[k] = struct{}{}
	}

	if p.Write {
		newWrites := make([]*node, 0)
		//if !varstate.wvc.leq(t1.vc) {
		//concurrent writes exist
		for i, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				newWrites = append(newWrites, varstate.writes[i])
				r := race{varstate.writes[i], newNode}
				//if isUnique(r) {
				if !intersect(w.ls, newNode.ls) {
					report.ReportRace(
						report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
						report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
						false, 0)
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
				//store race
				q := race{varstate.reads[i], newNode}

				if !intersect(r.ls, newNode.ls) {
					report.ReportRace(
						report.Location{File: q.acc1.ev.Ops[0].SourceRef, Line: q.acc1.ev.Ops[0].Line, W: q.acc1.ev.Ops[0].Kind&util.WRITE > 0},
						report.Location{File: q.acc2.ev.Ops[0].SourceRef, Line: q.acc2.ev.Ops[0].Line, W: q.acc2.ev.Ops[0].Kind&util.WRITE > 0},
						false, 0)
				}

			}
		}
		//}

		varstate.writes = newWrites
		t1.vc = t1.vc.add(p.T1, 1)
	} else if p.Read {
		//find concurrent writes, add wrd and store the detected race
		//	if !varstate.wvc.leq(t1.vc) {
		for i, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				r := race{varstate.writes[i], newNode}
				//if isUnique(r) {
				if !intersect(w.ls, newNode.ls) {
					report.ReportRace(
						report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
						report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
						false, 0)
				}

				//	}
			}
		}
		//}

		newReads := make([]*node, 0, len(varstate.reads))
		//if !varstate.rvc.leq(t1.vc) {
		for i, r := range varstate.reads {
			k := r.clock.get(r.ev.Thread)
			curr := t1.vc.get(r.ev.Thread)
			if k > curr {
				newReads = append(newReads, varstate.reads[i])
			}
		}
		//}
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
	}

	varstate.lastEv = p.Ev
	variables[p.T2] = varstate
	threads[p.T1] = t1
}

func (l *ListenerDataAccessHBNOLS) Put(p *util.SyncPair) {
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
	newNode := &node{ev: p.Ev, clock: t1.vc.clone()}

	if p.Write {
		newWrites := make([]*node, 0)
		//if !varstate.wvc.leq(t1.vc) {
		//concurrent writes exist
		for i, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				newWrites = append(newWrites, varstate.writes[i])
				r := race{varstate.writes[i], newNode}
				report.ReportRace(
					report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
					report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
					false, 0)
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
				//store race
				q := race{varstate.reads[i], newNode}

				report.ReportRace(
					report.Location{File: q.acc1.ev.Ops[0].SourceRef, Line: q.acc1.ev.Ops[0].Line, W: q.acc1.ev.Ops[0].Kind&util.WRITE > 0},
					report.Location{File: q.acc2.ev.Ops[0].SourceRef, Line: q.acc2.ev.Ops[0].Line, W: q.acc2.ev.Ops[0].Kind&util.WRITE > 0},
					false, 0)

			}
		}
		//}

		varstate.writes = newWrites
		t1.vc = t1.vc.add(p.T1, 1)
	} else if p.Read {
		//find concurrent writes, add wrd and store the detected race
		//	if !varstate.wvc.leq(t1.vc) {
		for i, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				r := race{varstate.writes[i], newNode}

				report.ReportRace(
					report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
					report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
					false, 0)

				//	}
			}
		}
		//}

		newReads := make([]*node, 0, len(varstate.reads))
		//if !varstate.rvc.leq(t1.vc) {
		for i, r := range varstate.reads {
			k := r.clock.get(r.ev.Thread)
			curr := t1.vc.get(r.ev.Thread)
			if k > curr {
				newReads = append(newReads, varstate.reads[i])
			}
		}
		//}
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
	}

	varstate.lastEv = p.Ev
	variables[p.T2] = varstate
	threads[p.T1] = t1
}

func (l *ListenerDataAccessHBNOP) Put(p *util.SyncPair) {
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
	newNode := &node{ev: p.Ev, clock: t1.vc.clone(), ls: make(map[uint32]struct{})}
	for k := range t1.ls {
		newNode.ls[k] = struct{}{}
	}

	if p.Write {
		newWrites := make([]*node, 0)
		//if !varstate.wvc.leq(t1.vc) {
		//concurrent writes exist
		for i, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				newWrites = append(newWrites, varstate.writes[i])
				r := race{varstate.writes[i], newNode}
				//if isUnique(r) {
				if !intersect(w.ls, newNode.ls) {
					report.ReportRace(
						report.Location{File: 1, Line: 1, W: true},
						report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
						false, 0)
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
				//store race
				q := race{varstate.reads[i], newNode}

				if !intersect(r.ls, newNode.ls) {
					report.ReportRace(
						report.Location{File: 1, Line: 1, W: false},
						report.Location{File: q.acc2.ev.Ops[0].SourceRef, Line: q.acc2.ev.Ops[0].Line, W: q.acc2.ev.Ops[0].Kind&util.WRITE > 0},
						false, 0)
				}

			}
		}
		//}

		varstate.writes = newWrites
		t1.vc = t1.vc.add(p.T1, 1)
	} else if p.Read {
		//find concurrent writes, add wrd and store the detected race
		//	if !varstate.wvc.leq(t1.vc) {
		for i, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				r := race{varstate.writes[i], newNode}
				//if isUnique(r) {
				if !intersect(w.ls, newNode.ls) {
					report.ReportRace(
						report.Location{File: 1, Line: 1, W: true},
						report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
						false, 0)
				}

				//	}
			}
		}
		//}

		newReads := make([]*node, 0, len(varstate.reads))
		//if !varstate.rvc.leq(t1.vc) {
		for i, r := range varstate.reads {
			k := r.clock.get(r.ev.Thread)
			curr := t1.vc.get(r.ev.Thread)
			if k > curr {
				newReads = append(newReads, varstate.reads[i])
			}
		}
		//}
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
	}

	varstate.lastEv = p.Ev
	variables[p.T2] = varstate
	threads[p.T1] = t1
}

var countFP = 0

var shbCountMap = make(map[string]struct{})

func (l *ListenerDataAccessSHB) Put(p *util.SyncPair) {
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
	newNode := &node{ev: p.Ev, clock: t1.vc.clone(), ls: make(map[uint32]struct{})}
	for k := range t1.ls {
		newNode.ls[k] = struct{}{}
	}
	if p.Write {
		varstate.lwNode = newNode
		newWrites := make([]*node, 0)
		//	if !varstate.wvc.leq(t1.vc) {
		//concurrent writes exist
		for i, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				newWrites = append(newWrites, varstate.writes[i])
				r := race{varstate.writes[i], newNode}

				if !intersect(w.ls, newNode.ls) {
					report.ReportRace(
						report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
						report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
						false, 0)
				}
				// if b {
				// 	b = intersect(w.ls, newNode.ls)
				// 	if b {
				// 		countFP++
				// 	} else {
				// 		s := fmt.Sprintf("%v:%v", newNode.ev.Ops[0].SourceRef, newNode.ev.Ops[0].Line)
				// 		shbCountMap[s] = struct{}{}
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }

			}
		}
		//		}
		newWrites = append(newWrites, newNode)
		//varstate.wvc = varstate.wvc.set(p.T1, t1.vc.get(p.T1))

		//		if !varstate.rvc.leq(t1.vc) {
		//concurrent reads exist
		for i, r := range varstate.reads {
			k := r.clock.get(r.ev.Thread)
			curr := t1.vc.get(r.ev.Thread)
			if k > curr {
				//store race
				q := race{varstate.reads[i], newNode}

				if !intersect(r.ls, newNode.ls) {
					report.ReportRace(
						report.Location{File: q.acc1.ev.Ops[0].SourceRef, Line: q.acc1.ev.Ops[0].Line, W: q.acc1.ev.Ops[0].Kind&util.WRITE > 0},
						report.Location{File: q.acc2.ev.Ops[0].SourceRef, Line: q.acc2.ev.Ops[0].Line, W: q.acc2.ev.Ops[0].Kind&util.WRITE > 0},
						false, 0)
				}

				// b := report.ReportRace(
				// 	report.Location{File: q.acc1.ev.Ops[0].SourceRef, Line: q.acc1.ev.Ops[0].Line, W: q.acc1.ev.Ops[0].Kind&util.WRITE > 0},
				// 	report.Location{File: q.acc2.ev.Ops[0].SourceRef, Line: q.acc2.ev.Ops[0].Line, W: q.acc2.ev.Ops[0].Kind&util.WRITE > 0},
				// 	false, 0)

				// if b {
				// 	b = intersect(r.ls, newNode.ls)
				// 	if b {
				// 		countFP++
				// 	} else {
				// 		s := fmt.Sprintf("%v:%v", newNode.ev.Ops[0].SourceRef, newNode.ev.Ops[0].Line)
				// 		shbCountMap[s] = struct{}{}
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }

			}
		}
		//		}

		varstate.writes = newWrites
		t1.vc = t1.vc.add(p.T1, 1)
	} else if p.Read {
		//find concurrent writes, add wrd and store the detected race
		//		if !varstate.wvc.leq(t1.vc) {
		for i, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				r := race{varstate.writes[i], newNode}
				//	if isUnique(r) {
				if !intersect(w.ls, newNode.ls) {
					report.ReportRace(
						report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
						report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
						false, 0)
				}

				// b := report.ReportRace(
				// 	report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
				// 	report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
				// 	false, 0)

				// if b {
				// 	b = intersect(w.ls, newNode.ls)
				// 	if b {
				// 		countFP++
				// 	} else {
				// 		s := fmt.Sprintf("%v:%v", newNode.ev.Ops[0].SourceRef, newNode.ev.Ops[0].Line)
				// 		shbCountMap[s] = struct{}{}
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }
				//	}
			}
		}
		//		}
		if varstate.lwNode != nil {
			t1.vc = t1.vc.ssync(varstate.lwNode.clock)
			newNode.clock = t1.vc.clone()
		}
		newReads := make([]*node, 0, len(varstate.reads))
		//		if !varstate.rvc.leq(t1.vc) {
		for i, r := range varstate.reads {
			k := r.clock.get(r.ev.Thread)
			curr := t1.vc.get(r.ev.Thread)
			if k > curr {
				newReads = append(newReads, varstate.reads[i])
			}
		}
		//		}
		newReads = append(newReads, newNode)

		//varstate.rvc = varstate.rvc.set(p.T1, t1.vc.get(p.T1))
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
	}

	varstate.lastEv = p.Ev
	variables[p.T2] = varstate
	threads[p.T1] = t1
}

func (l *ListenerDataAccessSHBNOLS) Put(p *util.SyncPair) {
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
	newNode := &node{ev: p.Ev, clock: t1.vc.clone(), ls: make(map[uint32]struct{})}
	for k := range t1.ls {
		newNode.ls[k] = struct{}{}
	}
	if p.Write {
		varstate.lwNode = newNode
		newWrites := make([]*node, 0)
		//	if !varstate.wvc.leq(t1.vc) {
		//concurrent writes exist
		for i, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				newWrites = append(newWrites, varstate.writes[i])
				r := race{varstate.writes[i], newNode}

				report.ReportRace(
					report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
					report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
					false, 0)

			}
		}
		//		}
		newWrites = append(newWrites, newNode)
		//varstate.wvc = varstate.wvc.set(p.T1, t1.vc.get(p.T1))

		//		if !varstate.rvc.leq(t1.vc) {
		//concurrent reads exist
		for i, r := range varstate.reads {
			k := r.clock.get(r.ev.Thread)
			curr := t1.vc.get(r.ev.Thread)
			if k > curr {
				//store race
				q := race{varstate.reads[i], newNode}

				report.ReportRace(
					report.Location{File: q.acc1.ev.Ops[0].SourceRef, Line: q.acc1.ev.Ops[0].Line, W: q.acc1.ev.Ops[0].Kind&util.WRITE > 0},
					report.Location{File: q.acc2.ev.Ops[0].SourceRef, Line: q.acc2.ev.Ops[0].Line, W: q.acc2.ev.Ops[0].Kind&util.WRITE > 0},
					false, 0)

			}
		}
		//		}

		varstate.writes = newWrites
		t1.vc = t1.vc.add(p.T1, 1)
	} else if p.Read {
		//find concurrent writes, add wrd and store the detected race
		//		if !varstate.wvc.leq(t1.vc) {
		for i, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				r := race{varstate.writes[i], newNode}
				//	if isUnique(r) {
				report.ReportRace(
					report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
					report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
					false, 0)

			}
		}
		//		}
		if varstate.lwNode != nil {
			t1.vc = t1.vc.ssync(varstate.lwNode.clock)
			newNode.clock = t1.vc.clone()
		}
		newReads := make([]*node, 0, len(varstate.reads))
		//		if !varstate.rvc.leq(t1.vc) {
		for i, r := range varstate.reads {
			k := r.clock.get(r.ev.Thread)
			curr := t1.vc.get(r.ev.Thread)
			if k > curr {
				newReads = append(newReads, varstate.reads[i])
			}
		}
		//		}
		newReads = append(newReads, newNode)

		//varstate.rvc = varstate.rvc.set(p.T1, t1.vc.get(p.T1))
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
	}

	varstate.lastEv = p.Ev
	variables[p.T2] = varstate
	threads[p.T1] = t1
}

func (l *ListenerDataAccessSHBSingle) Put(p *util.SyncPair) {
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

	if !p.Write && !p.Read {
		//atomic op
		vol, ok := volatiles[p.T2]
		if !ok {
			vol = newvc2()
		}
		t1.vc = t1.vc.ssync(vol)
		vol = t1.vc.clone()
		volatiles[p.T2] = vol
		t1.vc = t1.vc.add(p.T1, 1)
		threads[p.T1] = t1
		return
	}

	newNode := &node{ev: p.Ev, clock: t1.vc.clone(), ls: make(map[uint32]struct{})}
	for k := range t1.ls {
		newNode.ls[k] = struct{}{}
	}

	newFrontier := make([]*node, 0, len(varstate.writes))

	for i, w := range varstate.writes {

		k := w.clock.get(w.ev.Thread)
		curr := t1.vc.get(w.ev.Thread)
		if k > curr {
			newFrontier = append(newFrontier, varstate.writes[i])

			if w.ev.Ops[0].Kind&util.WRITE > 0 || newNode.ev.Ops[0].Kind&util.WRITE > 0 {
				r := race{varstate.writes[i], newNode}
				b := report.ReportRace(
					report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
					report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
					false, 0)
				if b {
					b = intersect(w.ls, newNode.ls)
					if b {
						countFP++
					}
					fmt.Println(">>>", b, countFP)
				}
			}
		}

	}

	if p.Write {
		varstate.lwNode = newNode
	} else if p.Read {
		if varstate.lwNode != nil {
			t1.vc = t1.vc.ssync(varstate.lwNode.clock)
			newNode.clock = t1.vc.clone()
		}
	}

	newFrontier = append(newFrontier, newNode)
	varstate.writes = newFrontier

	t1.vc = t1.vc.add(p.T1, 1)

	variables[p.T2] = varstate
	threads[p.T1] = t1

}

func (l *ListenerDataAccessSHBWrites) Put(p *util.SyncPair) {
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

	if !p.Write && !p.Read {
		//atomic op
		vol, ok := volatiles[p.T2]
		if !ok {
			vol = newvc2()
		}
		t1.vc = t1.vc.ssync(vol)
		vol = t1.vc.clone()
		volatiles[p.T2] = vol
		t1.vc = t1.vc.add(p.T1, 1)
		threads[p.T1] = t1
		return
	}

	newNode := &node{ev: p.Ev, clock: t1.vc.clone(), ls: make(map[uint32]struct{})}
	for k := range t1.ls {
		newNode.ls[k] = struct{}{}
	}

	newFrontier := make([]*node, 0, len(varstate.writes))

	for i, w := range varstate.writes {

		k := w.clock.get(w.ev.Thread)
		curr := t1.vc.get(w.ev.Thread)
		if k > curr {
			newFrontier = append(newFrontier, varstate.writes[i])

			if w.ev.Ops[0].Kind&util.WRITE > 0 || newNode.ev.Ops[0].Kind&util.WRITE > 0 {
				r := race{varstate.writes[i], newNode}
				b := report.ReportRace(
					report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
					report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
					false, 0)
				if b {
					b = intersect(w.ls, newNode.ls)
					if b {
						countFP++
					}
					fmt.Println(">>>", b, countFP)
				}
			}
		} else if w.ev.Ops[0].Kind&util.WRITE > 0 && p.Read { //read events do not replace writes!
			newFrontier = append(newFrontier, varstate.writes[i])
		}

	}

	if p.Write {
		varstate.lwNode = newNode
	} else if p.Read {
		if varstate.lwNode != nil {
			t1.vc = t1.vc.ssync(varstate.lwNode.clock)
			newNode.clock = t1.vc.clone()
		}
	}

	newFrontier = append(newFrontier, newNode)
	varstate.writes = newFrontier

	t1.vc = t1.vc.add(p.T1, 1)

	variables[p.T2] = varstate
	threads[p.T1] = t1

}

func (l *ListenerDataAccessSHBDefault) Put(p *util.SyncPair) {
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

	if !p.Write && !p.Read {
		//atomic op
		vol, ok := volatiles[p.T2]
		if !ok {
			vol = newvc2()
		}
		t1.vc = t1.vc.ssync(vol)
		vol = t1.vc.clone()
		volatiles[p.T2] = vol
		t1.vc = t1.vc.add(p.T1, 1)
		threads[p.T1] = t1
		return
	}

	if p.Write {
		if !varstate.wvc.leq(t1.vc) {
			report.ReportRace(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
		}
		if !varstate.rvc.leq(t1.vc) {
			report.ReportRace(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
		}

		varstate.lastWrite = t1.vc.clone()
		varstate.wvc = varstate.wvc.set(p.T1, t1.vc.get(p.T1))
	} else if p.Read {
		if !varstate.wvc.leq(t1.vc) {
			report.ReportRace(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
		}
		t1.vc = t1.vc.ssync(varstate.lastWrite)
		varstate.rvc = varstate.rvc.set(p.T1, t1.vc.get(p.T1))
	}

	t1.vc = t1.vc.add(p.T1, 1)

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
	t1.ls[p.T2] = struct{}{}
	t1.vc = t1.vc.ssync(lock.vc)
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

	signalList[p.T2] = signal{t1.vc.clone()}

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

func (l *ListenerDataAccessSHBNoPartner) Put(p *util.SyncPair) {
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
	newNode := &node{ev: p.Ev, clock: t1.vc.clone(), ls: make(map[uint32]struct{})}
	for k := range t1.ls {
		newNode.ls[k] = struct{}{}
	}
	if p.Write {
		varstate.lwNode = newNode
		newWrites := make([]*node, 0)
		//	if !varstate.wvc.leq(t1.vc) {
		//concurrent writes exist
		for _, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				newWrites = append(newWrites, w)

				if !intersect(w.ls, newNode.ls) {
					report.ReportRace(
						report.Location{File: 1, Line: 1, W: true},
						report.Location{File: newNode.ev.Ops[0].SourceRef, Line: newNode.ev.Ops[0].Line, W: newNode.ev.Ops[0].Kind&util.WRITE > 0},
						false, 0)
				}
				// if b {
				// 	b = intersect(w.ls, newNode.ls)
				// 	if b {
				// 		countFP++
				// 	} else {
				// 		s := fmt.Sprintf("%v:%v", newNode.ev.Ops[0].SourceRef, newNode.ev.Ops[0].Line)
				// 		shbCountMap[s] = struct{}{}
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }

			}
		}
		//		}
		newWrites = append(newWrites, newNode)
		//varstate.wvc = varstate.wvc.set(p.T1, t1.vc.get(p.T1))

		//		if !varstate.rvc.leq(t1.vc) {
		//concurrent reads exist
		for _, r := range varstate.reads {
			k := r.clock.get(r.ev.Thread)
			curr := t1.vc.get(r.ev.Thread)
			if k > curr {
				if !intersect(r.ls, newNode.ls) {
					report.ReportRace(
						report.Location{File: 1, Line: 1, W: true},
						report.Location{File: newNode.ev.Ops[0].SourceRef, Line: newNode.ev.Ops[0].Line, W: newNode.ev.Ops[0].Kind&util.WRITE > 0},
						false, 0)
				}

				// if b {
				// 	b = intersect(r.ls, newNode.ls)
				// 	if b {
				// 		countFP++
				// 	} else {
				// 		s := fmt.Sprintf("%v:%v", newNode.ev.Ops[0].SourceRef, newNode.ev.Ops[0].Line)
				// 		shbCountMap[s] = struct{}{}
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }

			}
		}
		//		}

		varstate.writes = newWrites
		t1.vc = t1.vc.add(p.T1, 1)
	} else if p.Read {
		//find concurrent writes, add wrd and store the detected race
		//		if !varstate.wvc.leq(t1.vc) {
		for _, w := range varstate.writes {
			k := w.clock.get(w.ev.Thread)
			curr := t1.vc.get(w.ev.Thread)
			if k > curr {
				//	if isUnique(r) {
				if !intersect(w.ls, newNode.ls) {
					report.ReportRace(
						report.Location{File: 1, Line: 1, W: true},
						report.Location{File: newNode.ev.Ops[0].SourceRef, Line: newNode.ev.Ops[0].Line, W: newNode.ev.Ops[0].Kind&util.WRITE > 0},
						false, 0)
				}

				// if b {
				// 	b = intersect(w.ls, newNode.ls)
				// 	if b {
				// 		countFP++
				// 	} else {
				// 		s := fmt.Sprintf("%v:%v", newNode.ev.Ops[0].SourceRef, newNode.ev.Ops[0].Line)
				// 		shbCountMap[s] = struct{}{}
				// 	}
				// 	fmt.Println(">>>", b, countFP)
				// }
				// //	}
			}
		}
		//		}
		if varstate.lwNode != nil {
			t1.vc = t1.vc.ssync(varstate.lwNode.clock)
			newNode.clock = t1.vc.clone()
		}
		newReads := make([]*node, 0, len(varstate.reads))
		//		if !varstate.rvc.leq(t1.vc) {
		for i, r := range varstate.reads {
			k := r.clock.get(r.ev.Thread)
			curr := t1.vc.get(r.ev.Thread)
			if k > curr {
				newReads = append(newReads, varstate.reads[i])
			}
		}
		//		}
		newReads = append(newReads, newNode)

		//varstate.rvc = varstate.rvc.set(p.T1, t1.vc.get(p.T1))
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
	}

	varstate.lastEv = p.Ev
	variables[p.T2] = varstate
	threads[p.T1] = t1
}

func (l *ListenerPostProcess) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}
	fmt.Println("SHB/FT #races:", len(shbCountMap))
}
