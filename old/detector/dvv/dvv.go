package dvv

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

//not used currently
type ListenerChanClose struct{}
type ListenerOpClosedChan struct{}
type ListenerSelect struct{}

type EventCollector struct{}

var listeners []traceReplay.EventListener

func (l *EventCollector) Put(m *traceReplay.Machine, p *util.SyncPair) {
	for _, l := range listeners {
		l.Put(m, p)
	}
}

func Init() {
	algos.RegisterDetector("dvv", &EventCollector{})
}

func init() {
	locks = make(map[uint64]varset)
	threads = make(map[uint64]varset)
	signalList = make(map[uint64]varset)
	variables = make(map[uint64]variable)

	versionLoc = make(map[dot]*util.Item)

	listeners = []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccess{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&traceReplay.Stepper{},
	}
}

type variable struct {
	current      uint
	dvvs         []dottedVV
	inactiveDVVs []dottedVV
}

func newVar() variable {
	return variable{0, make([]dottedVV, 0), make([]dottedVV, 0)}
}

var locks map[uint64]varset

type varset map[uint64]versionVector

func (v varset) sync(v2 varset) varset {
	v3 := newVarSet()

	// copy v to v3
	for k, vv := range v {
		vv3 := vv.clone()
		v3[k] = vv3
	}

	for k, vv := range v2 {
		vv3, ok := v3[k]
		if !ok {
			vv3 = newVersionVector()
		}
		vv3 = vv3.sync(vv)
		v3[k] = vv3
	}

	return v3
}

func (v varset) clone() varset {
	return v.sync(newVarSet())
}

func newVarSet() varset {
	return make(map[uint64]versionVector)
}

var threads map[uint64]varset
var signalList map[uint64]varset
var variables map[uint64]variable

var versionLoc map[dot]*util.Item

func (l *ListenerAsyncSnd) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.AsyncSend {
		return
	}

	t1 := m.Threads[p.T1]
	ev := t1.Peek()

	if ev.Ops[0].Mutex&util.LOCK == 0 { //just handle locks for the moment
		return
	}

	lock, ok := locks[p.T2]
	if !ok {
		lock = newVarSet()
	}

	t1Set, ok := threads[p.T1]
	if !ok {
		t1Set = newVarSet()
	}
	t1Set = t1Set.sync(lock)

	threads[p.T1] = t1Set
}
func (l *ListenerAsyncRcv) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.AsyncRcv {
		return
	}

	t1 := m.Threads[p.T1]
	ev := t1.Peek()

	if ev.Ops[0].Mutex&util.UNLOCK == 0 { //just handle locks for the moment
		return
	}

	lock, ok := locks[p.T2]
	if !ok {
		lock = newVarSet()
	}

	t1Set, ok := threads[p.T1]
	if !ok {
		t1Set = newVarSet()
	}
	lock = t1Set.sync(lock)

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

	tmp := t1.sync(t2)

	threads[p.T1] = tmp
	threads[p.T2] = tmp.clone() // clone so it is independent from t1
}

func (l *ListenerDataAccess) Put(m *traceReplay.Machine, p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	thread := m.Threads[p.T1]
	ev := thread.Peek()

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newVarSet()
	}

	t1VarHistory, ok := t1[p.T2]
	if !ok {
		t1VarHistory = newVersionVector()
	}

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar()
	}

	//	isAtomic := ev.Ops[0].Kind&(util.ATOMICREAD|util.ATOMICWRITE) > 0

	if ev.Ops[0].Kind&util.WRITE > 0 {
		newDot := dot{varstate.current}
		versionLoc[newDot] = ev
		varstate.current++

		newdvv := newDVV()
		newdvv.d = newDot
		newdvv.v = t1VarHistory.clone()
		for _, idvv := range varstate.inactiveDVVs {
			if newdvv.v.contains(idvv.d) { // cold storage contains a dvv for which the dot is in the vv of the new dvv, copy this vv into the new vv
				newdvv.v = newdvv.v.sync(idvv.v)
			}
		}

		//1. expansion phase: check which existing dvvs are after the current access and shift the dots
		newdvvs := make([]dottedVV, 0, len(varstate.dvvs)) //stores the new dvvs for the current variable
		for _, dvv := range varstate.dvvs {
			if t1VarHistory.contains(dvv.d) { //versionhistory of t1 for var x contains this dot, merge it into the new dvv
				newdvv.v = newdvv.v.sync(dvv.v)
				newdvv.v[dvv.d] = struct{}{} //shift dot of dvv into history of newdvv
				//move old dvv to cold storage
				varstate.inactiveDVVs = append(varstate.inactiveDVVs, dvv)
			} else {
				newdvvs = append(newdvvs, dvv)
			}
		}

		newdvvs = append(newdvvs, newdvv) // add new dvv

		varstate.dvvs = newdvvs

		//2. update thread vv for this variable!
		t1VarHistory = t1VarHistory.set(newDot)

		//3. report races by determining the complement of all dvvs with the one with the new dot
		if len(varstate.dvvs) > 1 { //concurrent versions exist!
			for _, dvv := range varstate.dvvs {
				if dvv.d != newdvv.d {
					//report race between the dots first
					ev1 := versionLoc[dvv.d]
					report.Race(&ev1.Ops[0], &ev.Ops[0], report.SEVERE)

					//build complement of their vvs, all events in the complement are in a race with the new dot
					cvv := newdvv.v.complement(dvv.v)
					for s := range cvv {
						ev1 := versionLoc[s]
						report.Race(&ev1.Ops[0], &ev.Ops[0], report.SEVERE)
					}
				}
			}
		}
	} else if ev.Ops[0].Kind&util.READ > 0 {

	}

	t1[p.T2] = t1VarHistory
	//fmt.Println(t1VarHistory)
	threads[p.T1] = t1
	variables[p.T2] = varstate
	//fmt.Println(varstate.dvvs)
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
		t1 = t1.sync(t2)
	}
	threads[p.T1] = t1
}

func (l *ListenerChanClose) Put(m *traceReplay.Machine, p *util.SyncPair) {

}
func (l *ListenerOpClosedChan) Put(m *traceReplay.Machine, p *util.SyncPair) {

}

type dot struct {
	uint
}

type versionVector map[dot]struct{}

func (v versionVector) clone() versionVector {
	nv := newVersionVector()
	nv = nv.sync(v)
	return nv
}

func (v versionVector) contains(d dot) bool {
	for s := range v {
		if s == d {
			return true
		}
	}
	return false
}

func (v versionVector) sync(v2 versionVector) versionVector {
	v3 := newVersionVector()
	for s := range v2 {
		v3[s] = struct{}{}
	}
	for s := range v {
		v3[s] = struct{}{}
	}
	return v3
}

func (v versionVector) set(d dot) versionVector {
	v2 := newVersionVector()
	v2[d] = struct{}{}
	return v2
}

func (v versionVector) complement(v2 versionVector) versionVector {
	v3 := newVersionVector()
	for s := range v2 {
		_, ok := v[s]
		if !ok { // s is in v2 but not in v
			v3[s] = struct{}{}
		}
	}
	return v3
}

func newVersionVector() versionVector {
	return make(map[dot]struct{})
}

type dottedVV struct {
	d dot
	v versionVector
}

func newDVV() dottedVV {
	return dottedVV{dot{0}, newVersionVector()}
}

func (dvv *dottedVV) shift(d dot) dottedVV {
	dvv2 := newDVV()

	// set new dot
	dvv2.d = d

	//shift old dot into dvv2s versionvector
	dvv2.v[dvv.d] = struct{}{}

	//copy version vector of dvv into the new dvv
	for s := range dvv.v {
		dvv2.v[s] = struct{}{}
	}

	return dvv2
}

func (dvv *dottedVV) happensBefore(v versionVector) bool {
	return v.contains(dvv.d)
}

func (dvv *dottedVV) expand(d dot, vv versionVector) dottedVV {
	dvv2 := newDVV()
	dvv2.d = d
	dvv2.v[dvv.d] = struct{}{} //shift old dot to vv

	for s := range dvv.v { //copy vv of old dvv into new
		dvv2.v[s] = struct{}{}
	}

	for v := range vv { //merge the vv of the thread into the vv of the new dvv
		dvv2.v[v] = struct{}{}
	}
	return dvv2
}

func (dvv *dottedVV) merge(d dot, dvv2 *dottedVV) dottedVV {
	dvv3 := newDVV()

	dvv3.d = d

	// shift dots of dvv and dvv2 into the new dvv3s version vector
	dvv3.v[dvv.d] = struct{}{}
	dvv3.v[dvv2.d] = struct{}{}

	for s := range dvv.v {
		dvv3.v[s] = struct{}{}
	}
	for s := range dvv2.v {
		dvv3.v[s] = struct{}{}
	}
	return dvv3
}

func (dvv dottedVV) String() string {
	return fmt.Sprintf("(%v)[%v]", dvv.d, dvv.v)
}
