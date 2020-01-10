package tsanSC2

import (
	"../../util"
	algos "../analysis"
	"../report"
	"../traceReplay"
)

//Phase 1
type ListenerAsyncSndP1 struct{}
type ListenerAsyncRcvP1 struct{}
type ListenerDataAccessP1 struct{}
type ListenerGoForkP1 struct{}
type ListenerGoWaitP1 struct{}
type ListenerNTP1 struct{}
type ListenerNTWTP1 struct{}
type ListenerPostProcessP1 struct{}

//Phase 2
type ListenerAsyncSndP2 struct{}
type ListenerAsyncRcvP2 struct{}
type ListenerDataAccessP2 struct{}
type ListenerGoForkP2 struct{}
type ListenerGoWaitP2 struct{}
type ListenerNTP2 struct{}
type ListenerNTWTP2 struct{}
type ListenerPostProcessP2 struct{}

//Phase 3
type ListenerAsyncSndP3 struct{}
type ListenerAsyncRcvP3 struct{}
type ListenerDataAccessP3 struct{}
type ListenerGoForkP3 struct{}
type ListenerGoWaitP3 struct{}
type ListenerNTP3 struct{}
type ListenerNTWTP3 struct{}
type ListenerPostProcessP3 struct{}

//WRDWCPW1
type ListenerAsyncSndWCPW1 struct{}
type ListenerAsyncRcvWCPW1 struct{}
type ListenerDataAccessWCPW1 struct{}

type EventCollector struct {
	listenersP1 []traceReplay.EventListener //critical section ordering
	listenersP2 []traceReplay.EventListener //must happens-before
	listenersP3 []traceReplay.EventListener //lockset+racedetection
	phase       uint
}

var lockClues map[uint32][][]int

type clue struct {
	id    uint32
	count int
	rels  []vcepoch
}

func (l *EventCollector) Put(p *util.SyncPair) {
	switch l.phase {
	case 1:
		for _, l := range l.listenersP1 {
			l.Put(p)
		}
	case 2:
		for _, l := range l.listenersP2 {
			l.Put(p)
		}
	case 3:
		for _, l := range l.listenersP3 {
			l.Put(p)
		}
	}

	if p.PostProcess {
		l.phase++
	}
}

//PHASE 1

func (l *ListenerAsyncRcvP1) Put(p *util.SyncPair) {
	if !p.Unlock {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	if len(t1.ls) > 0 { //remove lock from lockset
		t1.ls = t1.ls[:len(t1.ls)-1]
	}

	lock := locks[p.T2]
	lock.hb = lock.hb.ssync(t1.vc)
	nPair := vcPair{owner: p.T1, acq: lock.acq, rel: lock.hb.clone(), count: lock.count}
	lock.history = append(lock.history, nPair)

	for _, t := range threads {
		if t.id != p.T1 {
			cshist := t.csHistory[p.T2]
			cshist = append(cshist, nPair)
			t.csHistory[p.T2] = cshist
		}
	}
	cshist := csHistory[p.T2]
	cshist = append(cshist, nPair)
	csHistory[p.T2] = cshist

	clue, ok := lockClues[p.T2]
	if !ok {
		clue = make([][]int, 0)
		clue = append(clue, []int{})
		lockClues[p.T2] = clue
	}
	if len(clue) < (lock.count + 1) {
		//clue[lock.count] = make([]int, 0)
		clue = append(clue, []int{})
		lockClues[p.T2] = clue
	}

	lock.count++

	locks[p.T2] = lock
	t1.vc = t1.vc.add(p.T1, 1)
	threads[p.T1] = t1
}

func (l *ListenerAsyncSndP1) Put(p *util.SyncPair) {
	if !p.Lock {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	t1.ls = append(t1.ls, p.T2)
	lock, ok := locks[p.T2]
	if !ok {
		lock = newL()
	}
	//t1.vc = t1.vc.ssync(lock.rel)
	lock.acq = newEpoch(p.T1, t1.vc.get(p.T1))

	locks[p.T2] = lock
	t1.vc = t1.vc.add(p.T1, 1)
	threads[p.T1] = t1
}

func (l *ListenerGoForkP1) Put(p *util.SyncPair) {
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
func (l *ListenerGoWaitP1) Put(p *util.SyncPair) {
	if !p.IsWait {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	t2, ok := signalList[p.T2]
	if ok {
		t1.vc = t1.vc.ssync(t2)
		t1.vc = t1.vc.add(p.T1, 1)
		threads[p.T1] = t1
	}
}

func (l *ListenerDataAccessP1) Put(p *util.SyncPair) {
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
		varstate.lastWrite = t1.vc.clone()
		//varstate.lwDot = &dot{v: t1.vc.clone(), t: uint16(p.T1)}
	} else if p.Read {
		t1.vc = t1.vc.ssync(varstate.lastWrite)
		for _, k := range t1.ls {
			csHist := t1.csHistory[k]
			nCsHist := make([]vcPair, 0, len(csHist))
			for i := 0; i < len(csHist); i++ {
				p := csHist[i]

				localTimeForLastOwner := t1.vc.get(p.owner)
				relTimeForLastOwner := p.rel.get(p.owner)

				//read event is ordered within previous critical section
				if p.acq.v < localTimeForLastOwner && localTimeForLastOwner < relTimeForLastOwner {
					//t1.vc = t1.vc.ssync(p.rel)
					lk := locks[k]

					clue := lockClues[k]
					if len(clue) < (lk.count + 1) {
						clue = append(clue, []int{})
					}
					clue[lk.count] = append(clue[lk.count], p.count)

					lockClues[k] = clue
				}

				if !(relTimeForLastOwner < localTimeForLastOwner) {
					nCsHist = append(nCsHist, p)
				}
			}
			t1.csHistory[k] = nCsHist
		}
	} else {
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

func (l *ListenerNTP1) Put(p *util.SyncPair) {
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
	t1.vc = t1.vc.ssync(vc)
	notifies[p.T2] = vc

	t1.vc = t1.vc.add(p.T1, 1)
	threads[p.T1] = t1
}
func (l *ListenerNTWTP1) Put(p *util.SyncPair) {
	if !p.IsNTWT {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	if vc, ok := notifies[p.T2]; ok {
		t1.vc = t1.vc.ssync(vc)
		vc = t1.vc.clone().add(p.T1, 1)
		notifies[p.T2] = vc

		t1.vc = t1.vc.add(p.T1, 1)
		threads[p.T1] = t1
	}
}

func (l *ListenerPostProcessP1) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}
	threads = make(map[uint32]*thread)
	locks = make(map[uint32]*lock)
	signalList = make(map[uint32]vcepoch)
	variables = make(map[uint32]*variable)
	volatiles = make(map[uint32]vcepoch)
	notifies = make(map[uint32]vcepoch)
	csHistory = make(map[uint32][]vcPair)
}

//PHASE 1 END

//PHASE 2
func (l *ListenerAsyncRcvP2) Put(p *util.SyncPair) {
	if !p.Unlock {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	if len(t1.ls) > 0 { //remove lock from lockset
		t1.ls = t1.ls[:len(t1.ls)-1]
	}

	lock := locks[p.T2]

	lock.rels = append(lock.rels, t1.vc.clone())

	nPair := vcPair{owner: p.T1, acq: lock.acq, rel: t1.vc.clone(), count: lock.count}
	lock.history = append(lock.history, nPair)

	for _, t := range threads {
		if t.id != p.T1 {
			cshist := t.csHistory[p.T2]
			cshist = append(cshist, nPair)
			t.csHistory[p.T2] = cshist
		}
	}
	cshist := csHistory[p.T2]
	cshist = append(cshist, nPair)
	csHistory[p.T2] = cshist

	lock.count++

	locks[p.T2] = lock
	t1.vc = t1.vc.add(p.T1, 1)
	threads[p.T1] = t1
}

func (l *ListenerAsyncSndP2) Put(p *util.SyncPair) {
	if !p.Lock {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	t1.ls = append(t1.ls, p.T2)

	lock, ok := locks[p.T2]
	if !ok {
		lock = newL()
	}

	clues := lockClues[p.T2]
	if len(clues) > lock.count {
		countClues := clues[lock.count]

		for _, x := range countClues {
			t1.vc = t1.vc.ssync(lock.rels[x])
		}
	}

	lock.acq = newEpoch(p.T1, t1.vc.get(p.T1))

	locks[p.T2] = lock
	t1.vc = t1.vc.add(p.T1, 1)
	threads[p.T1] = t1
}

func (l *ListenerDataAccessP2) Put(p *util.SyncPair) {
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

		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))
			thi_at_j := t1.vc.get(uint32(f.t))

			if k > thi_at_j {
				newFrontier = append(newFrontier, f) // RW(x) =  {j#k | j#k ∈ RW(x) ∧ k > Th(i)[j]}
				if !intersect(newFE.ls, f.ls) {
					report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: true},
						report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
				}
			}
		}

		newFrontier = append(newFrontier, newFE) // ∪{i#Th(i)[i]}
		varstate.frontier = newFrontier

		varstate.lastWrite = t1.vc.clone()
		varstate.lwDot = newFE
	} else if p.Read {
		newFrontier := make([]*dot, 0, len(varstate.frontier))
		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))          //j#k
			thi_at_j := t1.vc.get(uint32(f.t)) //Th(i)[j]

			if k > thi_at_j {
				newFrontier = append(newFrontier, f) // RW(x) =  {j]k | j]k ∈ RW(x) ∧ k > Th(i)[j]}
				if f.write {
					if !intersect(newFE.ls, f.ls) {
						report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: true},
							report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
					}
				}
			} else if f.write {
				newFrontier = append(newFrontier, f)
			}
		}
		newFrontier = append(newFrontier, newFE) // ∪{i#Th(i)[i]}
		varstate.frontier = newFrontier

		t1.vc = t1.vc.ssync(varstate.lastWrite)
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

func (l *ListenerGoForkP2) Put(p *util.SyncPair) {
	if !p.IsFork {
		return
	}

	p1l := ListenerGoForkP1{}
	p1l.Put(p)
}

func (l *ListenerGoWaitP2) Put(p *util.SyncPair) {
	if !p.IsWait {
		return
	}

	p1l := ListenerGoWaitP1{}
	p1l.Put(p)
}

func (l *ListenerNTP2) Put(p *util.SyncPair) {
	if !p.IsNT {
		return
	}
	p1l := ListenerNTP1{}
	p1l.Put(p)
}

func (l *ListenerNTWTP2) Put(p *util.SyncPair) {
	if !p.IsNTWT {
		return
	}
	p1l := ListenerNTWTP1{}
	p1l.Put(p)
}

func (l *ListenerPostProcessP2) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}

	threads = make(map[uint32]*thread)
	//locks = make(map[uint32]*lock)
	signalList = make(map[uint32]vcepoch)
	variables = make(map[uint32]*variable)
	volatiles = make(map[uint32]vcepoch)
	notifies = make(map[uint32]vcepoch)

	for _, l := range locks {
		l.reset()
	}
}

//PHASE 2 END

// PHASE 3

func (l *ListenerAsyncRcvP3) Put(p *util.SyncPair) {
	if !p.Unlock {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newTP3(p.T1)
	}

	if len(t1.ls) > 0 { //remove lock from lockset
		t1.ls = t1.ls[:len(t1.ls)-1]
	}

	lock := locks[p.T2]

	lock.rels = append(lock.rels, t1.vc.clone())

	lock.count++

	locks[p.T2] = lock
	t1.vc = t1.vc.add(p.T1, 1)
	threads[p.T1] = t1
}

func (l *ListenerAsyncSndP3) Put(p *util.SyncPair) {
	if !p.Lock {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newTP3(p.T1)
	}

	t1.ls = append(t1.ls, p.T2)

	lock, ok := locks[p.T2]
	if !ok {
		lock = newL()
	}

	clues := lockClues[p.T2]
	if len(clues) > lock.count {
		countClues := clues[lock.count]

		for _, x := range countClues {
			t1.vc = t1.vc.ssync(lock.rels[x])
		}
	}

	lock.acq = newEpoch(p.T1, t1.vc.get(p.T1))

	cshist := csHistory[p.T2]

	if len(cshist) > lock.count {
		nPair := cshist[lock.count]

		for _, t := range threads {
			if t.id != p.T1 {
				cshist := t.csHistory[p.T2]
				cshist = append(cshist, nPair)
				t.csHistory[p.T2] = cshist
			}
		}
	}

	locks[p.T2] = lock
	t1.vc = t1.vc.add(p.T1, 1)
	threads[p.T1] = t1
}

func (l *ListenerDataAccessP3) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newTP3(p.T1)
	}
	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar()
	}

	varstate.current++
	newFE := &dot{v: t1.vc.clone(), int: varstate.current, t: uint16(p.T1),
		sourceRef: p.Ev.Ops[0].SourceRef, line: uint16(p.Ev.Ops[0].Line),
		write: p.Write, ls: make(map[uint32]struct{}), pos: p.Ev.LocalIdx}
	for _, k := range t1.ls { //copy lockset of thread
		newFE.ls[k] = struct{}{}
	}
	//build lockset promotion using cs history data

	for k := range locks {
		if _, ok := newFE.ls[k]; ok {
			continue
		}
		lk, ok := t1.csHistory[k]
		if !ok || len(lk) == 0 {
			continue
		}

		nhist := make([]vcPair, 0, len(lk))
		for _, p := range lk {
			localTimeForLastOwner := t1.vc.get(p.owner)
			if p.rel.get(p.owner) < localTimeForLastOwner {
				continue
			} else {
				relTimeForLastOwner := p.rel.get(t1.id)

				if p.acq.v < localTimeForLastOwner && t1.vc.get(t1.id) < relTimeForLastOwner {
					newFE.ls[k] = struct{}{}
				}

				//if p.rel.get(p.owner) > localTimeForLastOwner {
				nhist = append(nhist, p)
				//}
			}

		}
		t1.csHistory[k] = nhist
	}

	if p.Write {
		newFrontier := make([]*dot, 0, len(varstate.frontier))

		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))
			thi_at_j := t1.vc.get(uint32(f.t))

			if k > thi_at_j {
				newFrontier = append(newFrontier, f)

				if !intersect(newFE.ls, f.ls) {
					report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: true},
						report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
				}
			}
		}

		newFrontier = append(newFrontier, newFE)
		varstate.frontier = newFrontier
		varstate.lastWrite = t1.vc.clone()
		varstate.lwDot = newFE
	} else if p.Read {
		//check lastwrite Race
		if varstate.lwDot != nil {
			k := varstate.lwDot.v.get(uint32(varstate.lwDot.t)) //j#k
			thi_at_j := t1.vc.get(uint32(varstate.lwDot.t))     //Th(i)[j]

			if k > thi_at_j {
				if !intersect(newFE.ls, varstate.lwDot.ls) {
					report.ReportRace(report.Location{File: uint32(varstate.lwDot.sourceRef), Line: uint32(varstate.lwDot.line), W: true},
						report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
				}
			}
		}
		t1.vc = t1.vc.ssync(varstate.lastWrite)
		newFE.v = t1.vc.clone()

		newFrontier := make([]*dot, 0, len(varstate.frontier))

		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))
			thi_at_j := t1.vc.get(uint32(f.t))

			if k > thi_at_j {
				newFrontier = append(newFrontier, f)
				if f.write {
					if !intersect(newFE.ls, f.ls) {
						report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: true},
							report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
					}
				}
			} else {
				if f.write {
					newFrontier = append(newFrontier, f)
				}
			}
		}

		newFrontier = append(newFrontier, newFE)
		varstate.frontier = newFrontier
	}
	t1.vc = t1.vc.add(p.T1, 1)
	threads[p.T1] = t1
	variables[p.T2] = varstate
}

func (l *ListenerGoForkP3) Put(p *util.SyncPair) {
	if !p.IsFork {
		return
	}
	p1l := ListenerGoForkP1{}
	p1l.Put(p)
}

func (l *ListenerGoWaitP3) Put(p *util.SyncPair) {
	if !p.IsWait {
		return
	}
	p1l := ListenerGoWaitP1{}
	p1l.Put(p)
}

func (l *ListenerNTP3) Put(p *util.SyncPair) {
	if !p.IsNT {
		return
	}
	p1l := ListenerNTP1{}
	p1l.Put(p)
}

func (l *ListenerNTWTP3) Put(p *util.SyncPair) {
	if !p.IsNTWT {
		return
	}

	p1l := ListenerNTWTP1{}
	p1l.Put(p)
}
func (l *ListenerPostProcessP3) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}
}

//PHASE 3 END

func Init() {
	threads = make(map[uint32]*thread)
	locks = make(map[uint32]*lock)
	signalList = make(map[uint32]vcepoch)
	variables = make(map[uint32]*variable)
	volatiles = make(map[uint32]vcepoch)
	notifies = make(map[uint32]vcepoch)
	csHistory = make(map[uint32][]vcPair)
	lockClues = make(map[uint32][][]int)

	listenersp1 := []traceReplay.EventListener{
		&ListenerAsyncSndP1{},
		&ListenerAsyncRcvP1{},
		&ListenerDataAccessP1{},
		&ListenerGoForkP1{},
		&ListenerGoWaitP1{},
		&ListenerNTP1{},
		&ListenerNTWTP1{},
		&ListenerPostProcessP1{},
	}
	listenersp2 := []traceReplay.EventListener{
		&ListenerAsyncSndP2{},
		&ListenerAsyncRcvP2{},
		&ListenerDataAccessP2{},
		&ListenerGoForkP2{},
		&ListenerGoWaitP2{},
		&ListenerNTP2{},
		&ListenerNTWTP2{},
		&ListenerPostProcessP2{},
	}
	listenersp3 := []traceReplay.EventListener{
		&ListenerAsyncSndP3{},
		&ListenerAsyncRcvP3{},
		&ListenerDataAccessP3{},
		&ListenerGoForkP3{},
		&ListenerGoWaitP3{},
		&ListenerNTP3{},
		&ListenerNTWTP3{},
		&ListenerPostProcessP3{},
	}

	algos.RegisterDetector("tsanSC2", &EventCollector{listenersp1, listenersp2, listenersp3, 1})
	algos.RegisterDetector("tsanWRDW1", &EventCollector{listenersP1: listenersp1, listenersP2: listenersp2, phase: 1})

	listenersp1WCP := []traceReplay.EventListener{
		&ListenerAsyncSndP1{},
		&ListenerAsyncRcvP1{},
		&ListenerDataAccessP1{},
		&ListenerGoForkP1{},
		&ListenerGoWaitP1{},
		&ListenerNTP1{},
		&ListenerNTWTP1{},
		&ListenerPostProcessP1{},
	}
	listenersp2WCP := []traceReplay.EventListener{
		&ListenerAsyncSndWCPW1{},
		&ListenerAsyncRcvWCPW1{},
		&ListenerDataAccessWCPW1{},
		&ListenerGoForkP2{},
		&ListenerGoWaitP2{},
		&ListenerNTP2{},
		&ListenerNTWTP2{},
		&ListenerPostProcessP2{},
	}

	algos.RegisterDetector("tsanWRDWCP", &EventCollector{listenersP1: listenersp1WCP, listenersP2: listenersp2WCP, phase: 1})
}

var threads map[uint32]*thread
var locks map[uint32]*lock
var signalList map[uint32]vcepoch
var variables map[uint32]*variable
var volatiles map[uint32]vcepoch
var notifies map[uint32]vcepoch

var csHistory map[uint32][]vcPair

type thread struct {
	vc vcepoch
	//ls        map[uint32]struct{}
	ls        []uint32
	posLock   map[uint32]int
	csHistory map[uint32][]vcPair
	id        uint32
	deps      []intpair
}

func newT(id uint32) *thread {
	t := &thread{id: id, vc: newvc2().set(id, 1), ls: make([]uint32, 0), /*ls: make(map[uint32]struct{})*/
		posLock: make(map[uint32]int), csHistory: make(map[uint32][]vcPair)}

	if len(lockClues) > 0 {
		for k, l := range locks {
			list, ok := t.csHistory[k]
			if !ok {
				list = make([]vcPair, 0)
			}

			for i := 0; i < l.count; i++ {
				list = append(list, l.history[i])
			}

			t.csHistory[k] = list
		}
	} else {

		for k, l := range locks {
			list, ok := t.csHistory[k]
			if !ok {
				list = make([]vcPair, 0)
			}
			list = append(list, l.history...)

			t.csHistory[k] = list
		}
	}
	return t
}

func newTP3(id uint32) *thread {
	t := &thread{id: id, vc: newvc2().set(id, 1), ls: make([]uint32, 0), /*ls: make(map[uint32]struct{})*/
		posLock: make(map[uint32]int), csHistory: make(map[uint32][]vcPair)}
	for k, l := range locks {
		cshist := csHistory[k]

		list := t.csHistory[k]
		if len(cshist) > l.count+2 {
			list = append(list, cshist[:l.count+2]...)
		} else {
			list = append(list, cshist...)
		}

		t.csHistory[k] = list

	}
	return t
}

type lock struct {
	rel        vcepoch
	rels       []vcepoch
	hb         vcepoch
	history    []vcPair
	acq        epoch
	nextAcq    epoch
	count      int
	strongSync bool
}

func (l *lock) reset() {
	l.rel = newvc2()
	l.rels = make([]vcepoch, 0)
	l.hb = newvc2()
	l.count = 0
}

func newL() *lock {
	return &lock{rel: newvc2(), hb: newvc2(), history: make([]vcPair, 0), rels: make([]vcepoch, 0)}
}

type vcPair struct {
	owner uint32
	acq   epoch
	rel   vcepoch
	count int
}
type pair struct {
	*dot
	a bool
}

type intpair struct {
	lk    uint32
	count int
	vc    vcepoch
}

type read struct {
	File uint32
	Line uint32
	T    uint32
}

type variable struct {
	races       []datarace
	history     []variableHistory
	frontier    []*dot
	graph       *fsGraph
	lastWrite   vcepoch
	lwLocks     map[uint32]struct{}
	lwOpenLocks []intpair
	lwDot       *dot
	current     int
}

func newVar() *variable {
	return &variable{lastWrite: newvc2(), lwDot: nil, frontier: make([]*dot, 0),
		current: 0, graph: newGraph(), races: make([]datarace, 0),
		history: make([]variableHistory, 0), lwLocks: make(map[uint32]struct{})}
}

type dataRace struct {
	raceAcc *dot
	prevAcc *dot
	deps    []intpair
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

func intersect(a, b map[uint32]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; ok {
			return true
		}
	}
	return false
}

//WCPW1
func (l *ListenerAsyncRcvWCPW1) Put(p *util.SyncPair) {
	if !p.Unlock {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	if len(t1.ls) > 0 { //remove lock from lockset
		t1.ls = t1.ls[:len(t1.ls)-1]
	}

	lock := locks[p.T2]

	lock.rels = append(lock.rels, t1.vc.clone())

	nPair := vcPair{owner: p.T1, acq: lock.acq, rel: t1.vc.clone(), count: lock.count}
	lock.history = append(lock.history, nPair)

	for _, t := range threads {
		if t.id != p.T1 {
			cshist := t.csHistory[p.T2]
			cshist = append(cshist, nPair)
			t.csHistory[p.T2] = cshist
		}
	}
	cshist := csHistory[p.T2]
	cshist = append(cshist, nPair)
	csHistory[p.T2] = cshist

	lock.count++

	locks[p.T2] = lock
	t1.vc = t1.vc.add(p.T1, 1)
	threads[p.T1] = t1
}

func (l *ListenerAsyncSndWCPW1) Put(p *util.SyncPair) {
	if !p.Lock {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	t1.ls = append(t1.ls, p.T2)

	lock, ok := locks[p.T2]
	if !ok {
		lock = newL()
	}

	clues := lockClues[p.T2]
	if len(clues) > lock.count {
		countClues := clues[lock.count]

		for _, x := range countClues {
			t1.vc = t1.vc.ssync(lock.rels[x])
		}
	}

	lock.acq = newEpoch(p.T1, t1.vc.get(p.T1))

	locks[p.T2] = lock
	t1.vc = t1.vc.add(p.T1, 1)
	threads[p.T1] = t1
}

func (l *ListenerDataAccessWCPW1) Put(p *util.SyncPair) {
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

		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))
			thi_at_j := t1.vc.get(uint32(f.t))

			if k > thi_at_j {
				newFrontier = append(newFrontier, f) // RW(x) =  {j#k | j#k ∈ RW(x) ∧ k > Th(i)[j]}
				if !intersect(newFE.ls, f.ls) {
					report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: true},
						report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
				}
			}
		}

		newFrontier = append(newFrontier, newFE) // ∪{i#Th(i)[i]}
		varstate.frontier = newFrontier

		varstate.lastWrite = t1.vc.clone()
		varstate.lwDot = newFE
	} else if p.Read {
		newFrontier := make([]*dot, 0, len(varstate.frontier))
		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))          //j#k
			thi_at_j := t1.vc.get(uint32(f.t)) //Th(i)[j]

			if k > thi_at_j {
				newFrontier = append(newFrontier, f) // RW(x) =  {j]k | j]k ∈ RW(x) ∧ k > Th(i)[j]}
				if f.write {
					if !intersect(newFE.ls, f.ls) {
						report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: true},
							report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
					}
				}
			} else if f.write {
				newFrontier = append(newFrontier, f)
			}
		}

		t1.vc = t1.vc.ssync(varstate.lastWrite)
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
