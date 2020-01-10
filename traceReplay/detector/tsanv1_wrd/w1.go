package tsanwrd

import (
	"../../util"
	"../report"
)

type ListenerAsyncSndW1P1 struct{}
type ListenerAsyncRcvW1P1 struct{}
type ListenerDataAccessW1P1 struct{}
type ListenerPostProcessW1P1 struct{}
type ListenerAsyncSndW1P2 struct{}
type ListenerAsyncRcvW1P2 struct{}
type ListenerDataAccessW1P2 struct{}
type ListenerPostProcessW1P2 struct{}

var lockClues map[uint32][][]int

func (l *ListenerAsyncSndW1P1) Put(p *util.SyncPair) {
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

func (l *ListenerAsyncRcvW1P1) Put(p *util.SyncPair) {
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

func (l *ListenerDataAccessW1P1) Put(p *util.SyncPair) {
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

func (l *ListenerPostProcessW1P1) Put(p *util.SyncPair) {
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

	for _, l := range locks {
		l.reset()
	}
}

func (l *ListenerAsyncRcvW1P2) Put(p *util.SyncPair) {
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

func (l *ListenerAsyncSndW1P2) Put(p *util.SyncPair) {
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

func (l *ListenerDataAccessW1P2) Put(p *util.SyncPair) {
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
