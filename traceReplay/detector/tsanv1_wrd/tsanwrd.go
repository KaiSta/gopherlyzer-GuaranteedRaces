package tsanwrd

import (
	"fmt"

	"../../util"
	algos "../analysis"
	"../traceReplay"
)

// WRDs + WRD transfer over locks only

type ListenerAsyncSnd2 struct{}
type ListenerAsyncRcv2 struct{}

type ListenerDataAccessOneRunSound struct{}
type ListenerDataAccessWRDW1 struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}
type ListenerNT struct{}
type ListenerNTWT struct{}
type ListenerPostProcess struct{}
type ListenerPostProcess2 struct{}

type EventCollector struct {
	listeners     []traceReplay.EventListener
	preProcessing []traceReplay.EventListener
	phase         uint
}

//var lockClues map[uint32]bitset.BitSet

func (l *EventCollector) Put(p *util.SyncPair) {
	if l.phase == 2 {
		for _, l := range l.listeners {
			l.Put(p)
		}
	} else {
		for _, l := range l.preProcessing {
			l.Put(p)
		}

		if p.PostProcess {
			l.phase++
		}
		// 	t1, ok := threads[p.T1]
		// 	if !ok {
		// 		t1 = newT(p.T1)

		// 		for k, v := range csHistory {
		// 			localhist := t1.csHistory[k]
		// 			for _, x := range v {
		// 				localhist = append(localhist, x)
		// 			}
		// 			t1.csHistory[k] = localhist
		// 		}
		// 	}

		// 	if p.Lock {
		// 		//t1.ls[p.T2] = struct{}{}
		// 		t1.ls = append(t1.ls, p.T2)
		// 		lock, ok := locks[p.T2]
		// 		if !ok {
		// 			lock = newL()
		// 		}
		// 		//t1.vc = t1.vc.ssync(lock.rel)
		// 		lock.acq = newEpoch(p.T1, t1.vc.get(p.T1))

		// 		//maybe?
		// 		//	t1.vc = t1.vc.ssync(lock.rel)

		// 		locks[p.T2] = lock
		// 	} else if p.Unlock {
		// 		//delete(t1.ls, p.T2)
		// 		if len(t1.ls) > 0 { //remove lock from lockset
		// 			t1.ls = t1.ls[:len(t1.ls)-1]
		// 		}

		// 		lock := locks[p.T2]
		// 		lock.hb = lock.hb.ssync(t1.vc)
		// 		nPair := vcPair{owner: p.T1, acq: lock.acq, rel: t1.vc.clone()}
		// 		lock.history = append(lock.history, nPair)

		// 		for _, t := range threads {
		// 			if t.id != p.T1 {
		// 				cshist := t.csHistory[p.T2]
		// 				cshist = append(cshist, nPair)
		// 				t.csHistory[p.T2] = cshist
		// 			}
		// 		}
		// 		cshist := csHistory[p.T2]
		// 		cshist = append(cshist, nPair)
		// 		csHistory[p.T2] = cshist

		// 		lock.count++

		// 		//maybe?
		// 		//lock.rel = lock.rel.ssync(t1.vc)

		// 		locks[p.T2] = lock
		// 	} else if p.Write {
		// 		varstate, ok := variables[p.T2]
		// 		if !ok {
		// 			varstate = newVar()
		// 		}
		// 		varstate.lastWrite = t1.vc.clone()
		// 		varstate.lwDot = &dot{v: t1.vc.clone(), t: uint16(p.T1)}
		// 		variables[p.T2] = varstate
		// 	} else if p.Read {
		// 		varstate, ok := variables[p.T2]
		// 		if !ok {
		// 			varstate = newVar()
		// 		}
		// 		t1.vc = t1.vc.ssync(varstate.lastWrite)

		// 		for _, k := range t1.ls {
		// 			csHist := t1.csHistory[k]
		// 			nCsHist := make([]vcPair, 0, len(csHist))
		// 			for i := 0; i < len(csHist); i++ {
		// 				p := csHist[i]

		// 				localTimeForLastOwner := t1.vc.get(p.owner)
		// 				relTimeForLastOwner := p.rel.get(p.owner)

		// 				//read event is ordered within previous critical section
		// 				if p.acq.v < localTimeForLastOwner && localTimeForLastOwner < relTimeForLastOwner {
		// 					t1.vc = t1.vc.ssync(p.rel)
		// 					clue := lockClues[k]
		// 					lk := locks[k]
		// 					clue.Set(lk.count)
		// 					lockClues[k] = clue
		// 				}

		// 				if !(relTimeForLastOwner < localTimeForLastOwner) {
		// 					nCsHist = append(nCsHist, p)
		// 				}
		// 			}
		// 			t1.csHistory[k] = nCsHist
		// 		}
		// 	} else if p.IsFork {
		// 		signalList[p.T2] = t1.vc.clone()
		// 	} else if p.IsWait {
		// 		t2, ok := signalList[p.T2]
		// 		if ok {
		// 			t1.vc = t1.vc.ssync(t2)
		// 		}
		// 	} else if p.IsNT {
		// 		vc, ok := notifies[p.T2]
		// 		if !ok {
		// 			vc = newvc2()
		// 		}
		// 		vc = vc.ssync(t1.vc)
		// 		t1.vc = t1.vc.ssync(vc)
		// 		notifies[p.T2] = vc
		// 	} else if p.IsNTWT {
		// 		if vc, ok := notifies[p.T2]; ok {
		// 			t1.vc = t1.vc.ssync(vc)
		// 			vc = t1.vc.clone().add(p.T1, 1)
		// 			notifies[p.T2] = vc
		// 		}
		// 	} else if p.PostProcess {
		// 		threads = make(map[uint32]*thread)
		// 		//locks = make(map[uint32]lock)
		// 		signalList = make(map[uint32]vcepoch)
		// 		variables = make(map[uint32]*variable)
		// 		volatiles = make(map[uint32]vcepoch)
		// 		notifies = make(map[uint32]vcepoch)
		// 		csHistory = make(map[uint32][]vcPair)

		// 		for _, l := range locks {
		// 			l.reset()
		// 		}

		// 		//	csHistory = nil
		// 		l.phase = 2
		// 	}
		// 	t1.vc = t1.vc.add(p.T1, 1)
		// 	threads[p.T1] = t1
		// }
	}
}

func Init() {
	threads = make(map[uint32]*thread)
	locks = make(map[uint32]*lock)
	signalList = make(map[uint32]vcepoch)
	variables = make(map[uint32]*variable)
	volatiles = make(map[uint32]vcepoch)
	notifies = make(map[uint32]vcepoch)
	csHistory = make(map[uint32][]vcPair)
	lockClues = make(map[uint32][][]int)

	algos.RegisterDetector("tsanE", &EventCollector{[]traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerDataAccess{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}, nil, 2})

	algos.RegisterDetector("tsanEWRD", &EventCollector{[]traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		//	&ListenerDataAccess{},
		&ListenerDataAccessWRD{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}, nil, 2})

	algos.RegisterDetector("tsanEWCP", &EventCollector{[]traceReplay.EventListener{
		&ListenerAsyncSndWCP{},
		&ListenerAsyncRcvWCP{},
		&ListenerDataAccessWCP{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}, nil, 2})

	algos.RegisterDetector("tsanEW1", &EventCollector{[]traceReplay.EventListener{
		&ListenerAsyncSndW1P2{},
		&ListenerAsyncRcvW1P2{},
		//	&ListenerDataAccess{},
		&ListenerDataAccessWRD{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}, []traceReplay.EventListener{
		&ListenerAsyncSndW1P1{},
		&ListenerAsyncRcvW1P1{},
		&ListenerDataAccessW1P1{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcessW1P1{},
	}, 1})

	algos.RegisterDetector("tsanEE", &EventCollector{[]traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerDataAccessEE{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}, nil, 2})

	algos.RegisterDetector("tsanEEWRD", &EventCollector{[]traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerDataAccessWRDEE{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}, nil, 2})

	algos.RegisterDetector("tsanEEWCP", &EventCollector{[]traceReplay.EventListener{
		&ListenerAsyncSndWCP{},
		&ListenerAsyncRcvWCP{},
		&ListenerDataAccessWCPEE{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}, nil, 2})

	algos.RegisterDetector("tsanEEW1", &EventCollector{[]traceReplay.EventListener{
		&ListenerAsyncSndW1P2{},
		&ListenerAsyncRcvW1P2{},
		&ListenerDataAccessWRDEE{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}, []traceReplay.EventListener{
		&ListenerAsyncSndW1P1{},
		&ListenerAsyncRcvW1P1{},
		&ListenerDataAccessW1P1{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcessW1P1{},
	}, 1})

}

var threads map[uint32]*thread
var locks map[uint32]*lock
var signalList map[uint32]vcepoch
var variables map[uint32]*variable
var volatiles map[uint32]vcepoch
var notifies map[uint32]vcepoch

var csHistory map[uint32][]vcPair

var multiLock uint
var singleLock uint
var maxMulti int

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
	t1.vc = t1.vc.ssync(vc)

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
	fmt.Println("Multilock:", multiLock)
	fmt.Println("SingleLock:", singleLock)
	fmt.Println("MaxMulti:", maxMulti)

}
