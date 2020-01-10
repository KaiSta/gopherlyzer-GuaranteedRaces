package racequal

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
type ListenerDataAccess struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}
type ListenerPostProcess struct{}
type ListenerPreBranch struct{}
type ListenerPostBranch struct{}

var WithWRD = true

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
	locations = make(map[string]struct{})

	listeners := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccess{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerPostProcess{},
		&ListenerPreBranch{},
		&ListenerPostBranch{},
	}
	algos.RegisterDetector("racequal", &EventCollector{listeners})
}

var threads map[uint32]thread
var locks map[uint32]lock
var signalList map[uint32]vcepoch
var variables map[uint32]variable
var locations map[string]struct{}

type thread struct {
	th         vcepoch
	wrd        vcepoch
	openBranch uint
	lockset    map[uint32]bool
	lastSync   *util.Item
}
type lock struct {
	rel vcepoch
}
type variable struct {
	lastWrite vcepoch
	races     []datarace
	//cw        []*event
	//cr        []*event
	frontier    []*event
	lastWriteEv *event
	history     []*event
}

func newVar() variable {
	return variable{lastWrite: newvc2(), races: make([]datarace, 0),
		frontier: make([]*event, 0),
		//cw: make([]*event, 0), cr: make([]*event, 0),
		history: make([]*event, 0)}
}

func newThread(id uint32) thread {
	return thread{openBranch: 0, th: newvc2().set(id, 1),
		wrd: newvc2(), lockset: make(map[uint32]bool)}
}

type event struct {
	evtVC   vcepoch
	ev      *util.Item
	lockset map[uint32]bool
}

type datarace struct {
	int
	ev1 *event
	ev2 *event
}

func (l *ListenerAsyncRcv) Put(p *util.SyncPair) {
	if !p.AsyncRcv || !p.Unlock {
		return
	}

	mutex, ok := locks[p.T2]
	if !ok {
		mutex = lock{rel: newvc2()}
	}

	t1, ok := threads[p.T1]
	if !ok {
		//t1 = thread{openBranch: 0, th: newvc2().set(p.T1, 1), wrd: newvc2(), lockset: make(map[uint32]bool)}
		t1 = newThread(p.T1)
	}

	mutex.rel = t1.th.clone()

	t1.th = t1.th.add(p.T1, 1)

	//update lockset for t1
	t1.lockset[p.T2] = false

	t1.lastSync = p.Ev

	threads[p.T1] = t1
	locks[p.T2] = mutex
}

func (l *ListenerAsyncSnd) Put(p *util.SyncPair) {
	if !p.AsyncSend || !p.Lock {
		return
	}

	mutex, ok := locks[p.T2]
	if !ok {
		mutex = lock{rel: newvc2()}
	}

	t1, ok := threads[p.T1]
	if !ok {
		//t1 = thread{openBranch: 0, th: newvc2().set(p.T1, 1), wrd: newvc2(), lockset: make(map[uint32]bool)}
		t1 = newThread(p.T1)
	}

	t1.th = t1.th.ssync(mutex.rel)

	//update lockset for t1
	t1.lockset[p.T2] = true

	threads[p.T1] = t1
}

func (l *ListenerSync) Put(p *util.SyncPair) {
}

func intersection(ls1, ls2 map[uint32]bool) (ls3 map[uint32]bool) {
	ls3 = make(map[uint32]bool)

	for k, v := range ls1 {
		if v {
			v2 := ls2[k]
			if v2 {
				ls3[k] = true
			}
		}
	}

	return ls3
}

func count(ls map[uint32]bool) uint {
	var n uint

	for _, v := range ls {
		if v {
			n++
		}
	}
	return n
}

var helpful uint
var both uint
var all uint
var different uint

var shbMap = make(map[string]struct{})
var allMap = make(map[string]struct{})

func (l *ListenerDataAccess) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		//	t1 = thread{openBranch: 0, th: newvc2().set(p.T1, 1), wrd: newvc2(), lockset: make(map[uint32]bool)}
		t1 = newThread(p.T1)
	}

	varstate, ok := variables[p.T2]
	if !ok {
		//varstate = variable{lastWrite: newvc2(), races: make([]datarace, 0), cw: make([]*event, 0), cr: make([]*event, 0)}
		varstate = newVar()
	}

	if p.Write {
		newEvent := &event{ev: p.Ev, evtVC: t1.th.clone(),
			lockset: make(map[uint32]bool)}
		//clone thread lockset
		for k, v := range t1.lockset {
			newEvent.lockset[k] = v
		}

		//newCw := make([]*event, 0, len(varstate.cw))
		newFrontier := make([]*event, 0, len(varstate.frontier))

		for _, f := range varstate.frontier {
			k := f.evtVC.get(f.ev.Thread)
			thi_at_j := t1.th.get(f.ev.Thread)

			if k > thi_at_j {
				//newCw = append(newCw, f)
				newFrontier = append(newFrontier, f)
				b := report.RaceStatistics2(report.Location{File: f.ev.Ops[0].SourceRef, Line: f.ev.Ops[0].Line, W: f.ev.Ops[0].Kind&util.WRITE > 0},
					report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)

				if b {
					loc1 := fmt.Sprintf("%v:%v", f.ev.Ops[0].SourceRef, f.ev.Ops[0].Line)
					loc2 := fmt.Sprintf("%v:%v", p.Ev.Ops[0].SourceRef, p.Ev.Ops[0].Line)

					locations[loc1] = struct{}{}
					locations[loc2] = struct{}{}

					shbMap[loc2] = struct{}{}
					allMap[loc1] = struct{}{}
					allMap[loc2] = struct{}{}

					varstate.races = append(varstate.races,
						datarace{int: len(varstate.history), ev1: f, ev2: newEvent})
					//check locksets for further infos:
					t1ls := count(newEvent.lockset)
					t2ls := count(f.lockset)
					if t1ls == 0 && t2ls == 0 {
						//both access were not protected, so both lines need to be reported since both need lock protection
						fmt.Println("Both accesses unprotected!")
						both++
					} else if t1ls == 0 {
						helpful++
						fmt.Println("Current access is unprotected, FT/SHB would report the helpful access!")
					} else if t2ls == 0 {
						fmt.Println("previous access was unprotected, SHB/FT report the useless access!")
					} else {
						fmt.Println("protected by different locks!")
						different++
					}
					fmt.Println("Previous sync of current thread = ", t1.lastSync)
					all++
				}
			}
		}
		newFrontier = append(newFrontier, newEvent)
		varstate.frontier = newFrontier
		//newCw = append(newCw, newEvent)
		//varstate.cw = newCw

		if WithWRD {
			varstate.lastWrite = t1.th.clone()
			varstate.lastWriteEv = newEvent
		}
		t1.th = t1.th.add(p.T1, 1)
		varstate.history = append(varstate.history, newEvent)

	} else if p.Read {
		newEvent := &event{ev: p.Ev, evtVC: t1.th.union(varstate.lastWrite), lockset: make(map[uint32]bool)}
		//clone thread lockset
		for k, v := range t1.lockset {
			newEvent.lockset[k] = v
		}

		// if varstate.lastWriteEv != nil {
		// 	curVal := t1.th.get(varstate.lastWriteEv.ev.Thread)
		// 	lwVal := varstate.lastWrite.get(varstate.lastWriteEv.ev.Thread)
		// 	if lwVal > curVal {
		// 		report.RaceStatistics2(report.Location{File: varstate.lastWriteEv.ev.Ops[0].SourceRef, Line: varstate.lastWriteEv.ev.Ops[0].Line, W: true},
		// 			report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, true, 0)
		// 	}
		// }

		//t1.th = t1.th.ssync(varstate.lastWrite)

		//	newCr := make([]*event, 0, len(varstate.cr))
		newFrontier := make([]*event, 0, len(varstate.frontier))

		for _, f := range varstate.frontier {
			k := f.evtVC.get(f.ev.Thread)
			thi_at_j := t1.th.get(f.ev.Thread)

			if k > thi_at_j {
				newFrontier = append(newFrontier, f)

				if f.ev.Ops[0].Kind&util.WRITE > 0 {
					b := report.RaceStatistics2(report.Location{File: f.ev.Ops[0].SourceRef, Line: f.ev.Ops[0].Line, W: f.ev.Ops[0].Kind&util.WRITE > 0},
						report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, false, 0)

					if b {
						loc1 := fmt.Sprintf("%v:%v", f.ev.Ops[0].SourceRef, f.ev.Ops[0].Line)
						loc2 := fmt.Sprintf("%v:%v", p.Ev.Ops[0].SourceRef, p.Ev.Ops[0].Line)

						locations[loc1] = struct{}{}
						locations[loc2] = struct{}{}

						shbMap[loc2] = struct{}{}
						allMap[loc1] = struct{}{}
						allMap[loc2] = struct{}{}

						varstate.races = append(varstate.races,
							datarace{int: len(varstate.history), ev1: f, ev2: newEvent})
						//check locksets for further infos:
						t1ls := count(newEvent.lockset)
						t2ls := count(f.lockset)
						if t1ls == 0 && t2ls == 0 {
							//both access were not protected, so both lines need to be reported since both need lock protection
							fmt.Println("Both accesses unprotected!")
							both++
						} else if t1ls == 0 {
							helpful++
							fmt.Println("Current access is unprotected, FT/SHB would report the helpful access!")
						} else if t2ls == 0 {
							fmt.Println("previous access was unprotected, SHB/FT report the useless access")
						} else {
							fmt.Println("protected by different locks!")
							different++
						}
						fmt.Println("Previous sync of current thread = ", t1.lastSync)
						all++
					}
				}

			} else {
				if f.ev.Ops[0].Kind&util.WRITE > 0 {
					newFrontier = append(newFrontier, f)
				}
			}
		}

		newFrontier = append(newFrontier, newEvent)
		varstate.frontier = newFrontier

		if WithWRD {
			t1.th = t1.th.ssync(varstate.lastWrite)
		}

		t1.th = t1.th.add(p.T1, 1)
		varstate.history = append(varstate.history, newEvent)
	} else {
		//forgot atomics so ignore the panic here
		//	fmt.Println(p.Ev)
		//panic("Data access but not a write and not a read??")
	}

	threads[p.T1] = t1
	variables[p.T2] = varstate
}

func (l *ListenerGoFork) Put(p *util.SyncPair) {
	if !p.IsFork {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		//t1 = thread{openBranch: 0, th: newvc2().set(p.T1, 1), wrd: newvc2(), lockset: make(map[uint32]bool)}
		t1 = newThread(p.T1)
	}

	signalList[p.T2] = t1.th.clone()

	t1.th = t1.th.add(p.T1, 1)
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
			//t1 = thread{openBranch: 0, th: newvc2().set(p.T1, 1), wrd: newvc2(), lockset: make(map[uint32]bool)}
			t1 = newThread(p.T1)
		}

		t1.th = t1.th.ssync(t2)
		t1.th = t1.th.add(p.T1, 1)
		t1.lastSync = p.Ev
		threads[p.T1] = t1
	}
}

func (l *ListenerPreBranch) Put(p *util.SyncPair) {
	if !p.IsPreBranch {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		//t1 = thread{openBranch: 0, th: newvc2().set(p.T1, 1), wrd: newvc2(), lockset: make(map[uint32]bool)}
		t1 = newThread(p.T1)
	}

	t1.openBranch++
	threads[p.T1] = t1
}
func (l *ListenerPostBranch) Put(p *util.SyncPair) {
	if !p.IsPostBranch {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		//t1 = thread{openBranch: 0, th: newvc2().set(p.T1, 1), wrd: newvc2(), lockset: make(map[uint32]bool)}
		t1 = newThread(p.T1)
	}

	t1.openBranch--
	if t1.openBranch < 0 {
		panic("openBranch negative!")
	}
	threads[p.T1] = t1
}

func (l *ListenerPostProcess) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}

	fmt.Printf("Helpful:%v\nBoth:%v\nDifferent:%v\nAll:%v\n", helpful, both, different, all)

	distinctLocation := 0
	maxDistinctLocations := 0
	alreadyReported := 0
	checkedRaces := 0
	notConcurr := 0
	sameLocation := 0
	newLocation := 0
	newRace := 0

	report.Nocount = true

	newLocation2 := 0

	for _, v := range variables {
		for _, x := range v.races {
			checkedRaces++

			raceEv := v.history[x.int]
			raceEvIsWrite := raceEv.ev.Ops[0].Kind&util.WRITE > 0
			partnerLoc := fmt.Sprintf("%v:%v", x.ev1.ev.Ops[0].SourceRef,
				x.ev1.ev.Ops[0].Line)

			localDistinctLoc := 0

			for j := x.int; j >= 0; j-- {
				prevEv := v.history[j]
				prevEvIsWrite := prevEv.ev.Ops[0].Kind&util.WRITE > 0

				if !raceEvIsWrite && !prevEvIsWrite || raceEv.ev.Thread == prevEv.ev.Thread {
					continue
				}

				raceEvVal := raceEv.evtVC.get(prevEv.ev.Thread)
				prevEvVal := prevEv.evtVC.get(prevEv.ev.Thread)
				if prevEvVal > raceEvVal { //isConcurrent
					currLoc := fmt.Sprintf("%v:%v", prevEv.ev.Ops[0].SourceRef, prevEv.ev.Ops[0].Line)
					if _, ok := locations[currLoc]; !ok {
						newLocation++
					}

					b := report.RaceStatistics2(report.Location{File: prevEv.ev.Ops[0].SourceRef, Line: prevEv.ev.Ops[0].Line, W: prevEvIsWrite},
						report.Location{File: raceEv.ev.Ops[0].SourceRef, Line: raceEv.ev.Ops[0].Line, W: raceEvIsWrite}, false, 0)
					if b {
						loc1 := fmt.Sprintf("%v:%v", prevEv.ev.Ops[0].SourceRef, prevEv.ev.Ops[0].Line)

						if _, ok := allMap[loc1]; !ok {
							newLocation2++
						}
						allMap[loc1] = struct{}{}

						newRace++

						if currLoc != partnerLoc {
							distinctLocation++
							localDistinctLoc++
						} else {
							sameLocation++
						}
					} else {
						alreadyReported++
					}
				} else {
					notConcurr++
				}
			}
			if localDistinctLoc > maxDistinctLocations {
				maxDistinctLocations = localDistinctLoc
			}
		}
	}

	fmt.Printf("NewLocation(ALL):%v\nNewRace(ALL):%v\nDistinctLocations(ALL):%v\nSameLoc(ALL):%v\nAlreadyReported(ALL):%v\nMaxDistinctLoc:%v\nAvgDistinctLoc:%v\nNotConcurr:%v\n",
		newLocation, newRace, distinctLocation,
		sameLocation, alreadyReported, maxDistinctLocations,
		float64(distinctLocation)/float64(checkedRaces), notConcurr)

	fmt.Printf("New race locatoins compared to SHB: %v (%v/%v-(%v))\n", len(allMap)-len(shbMap), len(shbMap), len(allMap), newLocation2)
}
