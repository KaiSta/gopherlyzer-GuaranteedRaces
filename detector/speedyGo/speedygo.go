package speedygo

import (
	"fmt"

	"../../util"
	"../analysis"
	"../report"
	"../traceReplay"
)

var debug = false

type ListenerAsyncSnd struct{}
type ListenerAsyncRcv struct{}
type ListenerSync struct{}
type ListenerDataAccess struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}
type ListenerPostProcess struct{}

type EventCollector struct{}

var listeners []traceReplay.EventListener

func (l *EventCollector) Put(m *traceReplay.Machine, p *util.SyncPair) {
	for _, l := range listeners {
		l.Put(m, p)
	}
}

func Init() {
	algos.RegisterDetector("speedygo", &EventCollector{})
}

func init() {
	locks = make(map[uint64]varset)
	threads = make(map[uint64]varset)
	signalList = make(map[uint64]varset)
	variables = make(map[uint64]variable)
	threadLWs = make(map[uint64]varset)

	listeners = []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccess{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerPostProcess{},
		&traceReplay.Stepper{},
	}
}

func (l *ListenerAsyncSnd) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.AsyncSend {
		return
	}

	thread := m.Threads[p.T1]
	ev := thread.Peek()

	if ev.Ops[0].Mutex&util.LOCK == 0 {
		return
	}

	lock, ok := locks[p.T2]
	if !ok {
		lock = newVarSet()
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newVarSet()
	}

	if debug {
		fmt.Println("1>>>", lock)
		fmt.Println("1>>>", t1)
	}

	t1 = t1.union(lock)

	if debug {
		fmt.Println("2>>>", lock)
		fmt.Println("2>>>", t1)
	}

	threads[p.T1] = t1
}

func (l *ListenerAsyncRcv) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.AsyncRcv {
		return
	}

	thread := m.Threads[p.T1]
	ev := thread.Peek()

	if ev.Ops[0].Mutex&util.UNLOCK == 0 {
		return
	}

	lock, ok := locks[p.T2]
	if !ok {
		lock = newVarSet()
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newVarSet()
	}

	if debug {
		fmt.Println("1@@@", lock)
		fmt.Println("1@@@", t1)
	}

	lock = newVarSet()
	lock = lock.union(t1)

	if debug {
		fmt.Println("2@@@", lock)
		fmt.Println("2@@@", t1)
	}

	locks[p.T2] = lock
}

func (l *ListenerSync) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.Sync {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newVarSet()
	}

	t2, ok := threads[p.T2]
	if !ok {
		t2 = newVarSet()
	}

	tmp := t1.union(t2)

	threads[p.T1] = tmp
	threads[p.T2] = tmp.clone()

}

type datarace struct {
	ndot  dot
	rdots []dot
	vari  *variable
}

var dataraces []datarace

func (l *ListenerDataAccess) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.DataAccess {
		return
	}
	if v, ok := algos.Variables[p.T2]; ok {
		if !v.Shared {
			return
		}
	}

	thread := m.Threads[p.T1]
	ev := thread.Peek()

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newVarSet()
	}
	//fmt.Println("len_VarSet=", len(t1))
	t1VHistory, ok := t1[p.T2]
	if !ok {
		t1VHistory = newVV()
	}

	//fmt.Println("len_VV=", len(t1VHistory))
	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar()
	}

	racedots := make([]dot, 0, len(varstate.dots))

	isWrite := ev.Ops[0].Kind&util.WRITE > 0
	newVersion := dot(varstate.current) //version for this access
	if isWrite {
		varstate.versionInfo.Set(int(newVersion))
	}
	// else {
	// 	varstate.versionInfo.Clear(int(newVersion))
	// }
	varstate.current++ //version for the next access

	if debug {
		fmt.Println("1@@@", t1)
		fmt.Println("1>>>", varstate.dots)
	}

	varstate.versionLoc[newVersion] = report.Location{File: ev.Ops[0].SourceRef, Line: ev.Ops[0].Line} //event location for the current access

	// Find direct races with the dots in the variable and vv of the thread; add dots that need to be kept in the current varstate
	// to the new dot collection (TODO: if its a write, only remove write dots, read only read dots. [racedots currently only stores racy accesses!!!! important!!])

	// for _, d := range varstate.dots { //all dots stored in the variable
	// 	if !t1VHistory.contains(d) && !t2VHistory.contains(d) { //check if versionvector of the thread contains this dot
	// 		if varstate.versionInfo.Get(int(newVersion)) || varstate.versionInfo.Get(int(d)) { //at least one must be a write for a data race
	// 			report.Race2(varstate.versionLoc[d], varstate.versionLoc[newVersion], report.SEVERE)
	// 		}
	// 		racedots = append(racedots, d) //events that are not in the version vector of the thread need to be kept in the version vector of the variable
	// 	}
	// }
	for _, d := range varstate.dots { //all dots stored in the variable
		if t1VHistory.contains(d) {
			continue
		}
		found := false
		for _, v := range threadLWs {
			t2VHistory, ok := v[p.T2]
			if ok && t2VHistory.contains(d) {
				found = true
				break
			}
		}
		if !found {
			if varstate.versionInfo.Get(int(newVersion)) || varstate.versionInfo.Get(int(d)) { //at least one must be a write for a data race
				report.Race2(varstate.versionLoc[d], varstate.versionLoc[newVersion], report.SEVERE)
			}
			racedots = append(racedots, d)
		}
	}
	if len(racedots) > 0 {
		dataraces = append(dataraces, datarace{newVersion, racedots, &varstate})
	}
	//racedots contains all dots that are in a race with the current access dot at this moment

	//fmt.Println("detectRaces...")
	//detectGraphRaces2(newVersion, racedots, &varstate) //check graph for all races after finding races with the versions stored in newDots

	//fmt.Println("UpdatingVarStates...")
	newDots := append([]dot{}, newVersion) //newDots is going to contain the new dots of this variable
	for _, d := range racedots {           //iterate over the current dots
		if !isWrite && d == varstate.lwDot {
			continue
		}
		newDots = append(newDots, d)
	}

	varstate.dots = newDots

	if !isWrite {
		t1VHistory.add(varstate.lwDot)
		varstate.lwDot = 0
	}

	//fmt.Println("updateGraph...")
	updateGraph2(newVersion, t1VHistory, &varstate) //add connections from the current version to the known versions of the accessing thread

	//Update thread versionvector
	t1VHistory = newVV()
	t1VHistory.add(newVersion)
	t1[p.T2] = t1VHistory

	if isWrite {
		//update last write
		varstate.lastWrite = t1.clone()
		varstate.lwDot = newVersion
		varstate.lwT = p.T1
	} else {
		//noch nach threads aufteilen!!
		threadLWs[varstate.lwT] = varstate.lastWrite
		//t1 = t1.union(varstate.lastWrite)
	}

	threads[p.T1] = t1
	variables[p.T2] = varstate

	if debug {
		fmt.Println("2@@@", t1)
		fmt.Println("2>>>", varstate.dots)
	}
}

func (l *ListenerGoFork) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.IsFork {
		return
	}

	t1, ok := threads[p.T1]

	if !ok {
		t1 = newVarSet()
	}

	signalList[p.T2] = t1.clone()
}

func (l *ListenerGoWait) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.IsWait {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newVarSet()
	}

	t2, ok := signalList[p.T2]
	if ok {
		t1 = t1.union(t2)
	}
	threads[p.T1] = t1
}

func (l *ListenerPostProcess) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.PostProcess {
		return
	}

	for _, dr := range dataraces {
		if debug {
			fmt.Println(dr.ndot, ":", dr.rdots)
		}
		detectGraphRaces2(dr.ndot, dr.rdots, dr.vari)
	}

}
