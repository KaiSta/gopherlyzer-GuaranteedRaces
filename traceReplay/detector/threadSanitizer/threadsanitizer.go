package threadSanitizer

import (
	"../../util"
	algos "../analysis"
	"../report"
	"../traceReplay"
)

type ListenerAsyncSnd struct{}
type ListenerAsyncRcv struct{}
type ListenerChanClose struct{}
type ListenerOpClosedChan struct{}
type ListenerSync struct{}
type ListenerDataAccess struct{}
type ListenerDataAccess2 struct{}
type ListenerDataAccessWRD struct{}
type ListenerSelect struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}
type ListenerNT struct{}
type ListenerNTWT struct{}
type ListenerPostProcess struct{}

var threads map[uint32]thread
var signalList map[uint32]vcepoch
var variableMap map[uint32]accessHistory
var volatiles map[uint32]vcepoch
var notifies map[uint32]vcepoch

type accessEvent struct {
	VC      vcepoch
	Lockset map[uint32]struct{}
	Ev      *util.Item
}

type accessHistory struct {
	write     []accessEvent
	read      []accessEvent
	lastWrite accessEvent
}

type thread struct {
	set map[uint32]struct{}
	vc  vcepoch
}

func newThread() thread {
	return thread{make(map[uint32]struct{}), newvc2()}
}

type EventCollector struct{ listeners []traceReplay.EventListener }

func (l *EventCollector) Put(p *util.SyncPair) {
	for _, l := range l.listeners {
		l.Put(p)
	}
}

func Init() {
	variableMap = make(map[uint32]accessHistory)
	threads = make(map[uint32]thread)
	signalList = make(map[uint32]vcepoch)
	volatiles = make(map[uint32]vcepoch)
	notifies = make(map[uint32]vcepoch)

	listeners := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccess2{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("tsan", &EventCollector{listeners})

	listeners2 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccessWRD{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("tsanWRD", &EventCollector{listeners2})
}

func (l *ListenerAsyncSnd) Put(p *util.SyncPair) {
	if !p.AsyncSend {
		return
	}
	if !p.Lock {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = thread{vc: newvc2().set(p.T1, 1), set: make(map[uint32]struct{})}
	}

	//thread owns this lock now move it to the lockset
	t1.set[p.T2] = struct{}{}
	t1.vc = t1.vc.add(p.T1, 1)

	//update the traceReplay.Machine state
	threads[p.T1] = t1
}

func (l *ListenerAsyncRcv) Put(p *util.SyncPair) {
	if !p.AsyncRcv {
		return
	}

	if !p.Unlock {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = thread{vc: newvc2().set(p.T1, 1), set: make(map[uint32]struct{})}
	}

	//remove lock from lockset
	delete(t1.set, p.T2)
	t1.vc = t1.vc.add(p.T1, 1)

	//update the traceReplay.Machine state
	threads[p.T1] = t1
}

func (l *ListenerSync) Put(p *util.SyncPair) {
	if !p.Sync {
		return
	}
	// t1, ok := threads[p.T1]
	// if !ok {
	// 	t1 = newThread()
	// }
	// t2, ok := threads[p.T2]
	// if !ok {
	// 	t2 = newThread()
	// }

	// t1.vc.Add(p.T1, 1)
	// t2.vc.Add(p.T2, 1)

	// t1.vc.Sync(t2.vc)

	// threads[p.T1] = t1
	// threads[p.T2] = t2
}

func (l *ListenerDataAccess) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = thread{vc: newvc2().set(p.T1, 1), set: make(map[uint32]struct{})}
	}

	t1.vc = t1.vc.add(p.T1, 1)

	//clone threads lockset (sadly necessary...)
	lockset := make(map[uint32]struct{})
	for k := range t1.set {
		lockset[k] = struct{}{}
	}

	//clean the history, remove all previous access
	newHistory := accessHistory{}
	oldHistory := variableMap[p.T2]

	if p.Write {
		for _, k := range oldHistory.write {
			if !k.VC.less(t1.vc) {
				newHistory.write = append(newHistory.write, k)
			}
		}
		for _, k := range oldHistory.read {
			if !k.VC.less(t1.vc) {
				newHistory.read = append(newHistory.read, k)
			}
		}
		newHistory.write = append(newHistory.write, accessEvent{VC: t1.vc.clone(), Lockset: lockset, Ev: p.Ev})
	} else if p.Read {
		newHistory.write = oldHistory.write
		for _, k := range oldHistory.read {
			if !k.VC.less(t1.vc) {
				newHistory.read = append(newHistory.read, k)
			}
		}
		newHistory.read = append(newHistory.read, accessEvent{VC: t1.vc.clone(), Lockset: lockset, Ev: p.Ev})
	} else { //volatile synchronize
		vol, ok := volatiles[p.T2]
		if !ok {
			vol = newvc2()
		}
		t1.vc = t1.vc.ssync(vol)
		vol = t1.vc.clone()
		volatiles[p.T2] = vol
	}

	//compare the remaining concurrent events if the lockset is empty or not
	for i := 0; i < len(newHistory.write); i++ {
		ilock := newHistory.write[i].Lockset
		//check write-write pairs
		for j := i + 1; j < len(newHistory.write); j++ {
			if newHistory.write[i].VC.less(newHistory.write[j].VC) || newHistory.write[j].VC.less(newHistory.write[i].VC) {
				continue
			}
			jlock := newHistory.write[j].Lockset
			if len(Intersection(ilock, jlock)) == 0 {
				report.ReportRace(report.Location{File: newHistory.write[i].Ev.Ops[0].SourceRef, Line: newHistory.write[i].Ev.Ops[0].Line, W: true},
					report.Location{File: newHistory.write[j].Ev.Ops[0].SourceRef, Line: newHistory.write[j].Ev.Ops[0].Line, W: true}, false, 0)

				//report.Race(&newHistory.write[j].Ev.Ops[0], &newHistory.write[i].Ev.Ops[0], report.SEVERE)
			}
		}

		for j := 0; j < len(newHistory.read); j++ {
			if newHistory.write[i].VC.less(newHistory.read[j].VC) || newHistory.read[j].VC.less(newHistory.write[i].VC) {
				continue
			}
			jlock := newHistory.read[j].Lockset
			if len(Intersection(ilock, jlock)) == 0 {

				report.ReportRace(report.Location{File: newHistory.write[i].Ev.Ops[0].SourceRef, Line: newHistory.write[i].Ev.Ops[0].Line, W: true},
					report.Location{File: newHistory.read[j].Ev.Ops[0].SourceRef, Line: newHistory.read[j].Ev.Ops[0].Line, W: false}, false, 0)
				//report.Race(&newHistory.read[j].Ev.Ops[0], &newHistory.write[i].Ev.Ops[0], report.SEVERE)

			}
		}
	}

	threads[p.T1] = t1
	variableMap[p.T2] = newHistory
}

func (l *ListenerDataAccess2) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = thread{vc: newvc2().set(p.T1, 1), set: make(map[uint32]struct{})}
	}

	t1.vc = t1.vc.add(p.T1, 1)

	//clone threads lockset (sadly necessary...)
	lockset := make(map[uint32]struct{})
	for k := range t1.set {
		lockset[k] = struct{}{}
	}

	//clean the history, remove all previous access
	newHistory := accessHistory{}
	oldHistory := variableMap[p.T2]

	if p.Write {
		for _, k := range oldHistory.write {
			if !k.VC.less(t1.vc) {
				newHistory.write = append(newHistory.write, k)
			}
		}
		for _, k := range oldHistory.read {
			if !k.VC.less(t1.vc) {
				newHistory.read = append(newHistory.read, k)
			}
		}

		newAccessEvent := accessEvent{VC: t1.vc.clone(), Lockset: lockset, Ev: p.Ev}

		for j := 0; j < len(newHistory.write); j++ {
			if newAccessEvent.VC.less(newHistory.write[j].VC) || newHistory.write[j].VC.less(newAccessEvent.VC) {
				continue
			}
			jlock := newHistory.write[j].Lockset
			if len(Intersection(newAccessEvent.Lockset, jlock)) == 0 {
				report.ReportRace(
					report.Location{File: newHistory.write[j].Ev.Ops[0].SourceRef, Line: newHistory.write[j].Ev.Ops[0].Line, W: true},
					report.Location{File: newAccessEvent.Ev.Ops[0].SourceRef, Line: newAccessEvent.Ev.Ops[0].Line, W: true}, false, 0)
			}
		}

		for j := 0; j < len(newHistory.read); j++ {
			if newAccessEvent.VC.less(newHistory.read[j].VC) || newHistory.read[j].VC.less(newAccessEvent.VC) {
				continue
			}
			jlock := newHistory.read[j].Lockset
			if len(Intersection(newAccessEvent.Lockset, jlock)) == 0 {

				report.ReportRace(
					report.Location{File: newHistory.read[j].Ev.Ops[0].SourceRef, Line: newHistory.read[j].Ev.Ops[0].Line, W: false},
					report.Location{File: newAccessEvent.Ev.Ops[0].SourceRef, Line: newAccessEvent.Ev.Ops[0].Line, W: true}, false, 0)

			}
		}

		newHistory.write = append(newHistory.write, newAccessEvent)
	} else if p.Read {
		newHistory.write = oldHistory.write
		for _, k := range oldHistory.read {
			if !k.VC.less(t1.vc) {
				newHistory.read = append(newHistory.read, k)
			}
		}

		newAccessEvent := accessEvent{VC: t1.vc.clone(), Lockset: lockset, Ev: p.Ev}

		for j := 0; j < len(oldHistory.write); j++ {
			if newAccessEvent.VC.less(oldHistory.write[j].VC) || oldHistory.write[j].VC.less(newAccessEvent.VC) {
				continue
			}
			jlock := oldHistory.write[j].Lockset
			if len(Intersection(newAccessEvent.Lockset, jlock)) == 0 {
				report.ReportRace(
					report.Location{File: oldHistory.write[j].Ev.Ops[0].SourceRef, Line: oldHistory.write[j].Ev.Ops[0].Line, W: true},
					report.Location{File: newAccessEvent.Ev.Ops[0].SourceRef, Line: newAccessEvent.Ev.Ops[0].Line, W: false}, false, 0)
			}
		}

		newHistory.read = append(newHistory.read, newAccessEvent)
	} else { //volatile synchronize
		vol, ok := volatiles[p.T2]
		if !ok {
			vol = newvc2()
		}
		t1.vc = t1.vc.ssync(vol)
		vol = t1.vc.clone()
		volatiles[p.T2] = vol
	}

	threads[p.T1] = t1
	variableMap[p.T2] = newHistory
}

func Intersection(m1, m2 map[uint32]struct{}) map[uint32]struct{} {
	m3 := make(map[uint32]struct{})

	for k1 := range m1 {
		_, ok := m2[k1]
		if ok {
			m3[k1] = struct{}{}
		}
	}
	return m3
}

func (l *ListenerGoFork) Put(p *util.SyncPair) {
	if !p.IsFork {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = thread{vc: newvc2().set(p.T1, 1), set: make(map[uint32]struct{})}
	}

	signalList[p.T2] = t1.vc.clone()

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
			t1 = thread{vc: newvc2().set(p.T1, 1), set: make(map[uint32]struct{})}
		}
		t1.vc = t1.vc.ssync(vc)
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
		t1 = thread{vc: newvc2().set(p.T1, 1), set: make(map[uint32]struct{})}
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
			t1 = thread{vc: newvc2().set(p.T1, 1), set: make(map[uint32]struct{})}
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
	report.ReportFinish()
}

func (l *ListenerDataAccessWRD) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = thread{vc: newvc2().set(p.T1, 1), set: make(map[uint32]struct{})}
	}

	t1.vc = t1.vc.add(p.T1, 1)

	//clone threads lockset (sadly necessary...)
	lockset := make(map[uint32]struct{})
	for k := range t1.set {
		lockset[k] = struct{}{}
	}

	//clean the history, remove all previous access
	oldHistory := variableMap[p.T2]
	newHistory := accessHistory{lastWrite: oldHistory.lastWrite}

	if p.Write {
		for _, k := range oldHistory.write {
			if !k.VC.less(t1.vc) {
				newHistory.write = append(newHistory.write, k)
			}
		}
		for _, k := range oldHistory.read {
			if !k.VC.less(t1.vc) {
				newHistory.read = append(newHistory.read, k)
			}
		}

		newAccessEvent := accessEvent{VC: t1.vc.clone(), Lockset: lockset, Ev: p.Ev}

		for j := 0; j < len(newHistory.write); j++ {
			if newAccessEvent.VC.less(newHistory.write[j].VC) || newHistory.write[j].VC.less(newAccessEvent.VC) {
				continue
			}
			jlock := newHistory.write[j].Lockset
			if len(Intersection(newAccessEvent.Lockset, jlock)) == 0 {
				report.ReportRace(
					report.Location{File: newHistory.write[j].Ev.Ops[0].SourceRef, Line: newHistory.write[j].Ev.Ops[0].Line, W: true},
					report.Location{File: newAccessEvent.Ev.Ops[0].SourceRef, Line: newAccessEvent.Ev.Ops[0].Line, W: true}, false, 0)
			}
		}

		for j := 0; j < len(newHistory.read); j++ {
			if newAccessEvent.VC.less(newHistory.read[j].VC) || newHistory.read[j].VC.less(newAccessEvent.VC) {
				continue
			}
			jlock := newHistory.read[j].Lockset
			if len(Intersection(newAccessEvent.Lockset, jlock)) == 0 {

				report.ReportRace(
					report.Location{File: newHistory.read[j].Ev.Ops[0].SourceRef, Line: newHistory.read[j].Ev.Ops[0].Line, W: false},
					report.Location{File: newAccessEvent.Ev.Ops[0].SourceRef, Line: newAccessEvent.Ev.Ops[0].Line, W: true}, false, 0)

			}
		}

		newHistory.write = append(newHistory.write, newAccessEvent)
		newHistory.lastWrite = newAccessEvent
	} else if p.Read {
		//check for wrd races
		if oldHistory.lastWrite.VC != nil && !oldHistory.lastWrite.VC.less(t1.vc) && len(Intersection(oldHistory.lastWrite.Lockset, t1.set)) == 0 {
			report.ReportRace(
				report.Location{File: oldHistory.lastWrite.Ev.Ops[0].SourceRef, Line: oldHistory.lastWrite.Ev.Ops[0].Line, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, false, 0)
		}
		t1.vc = t1.vc.ssync(oldHistory.lastWrite.VC)

		newHistory.write = oldHistory.write
		for _, k := range oldHistory.read {
			if !k.VC.less(t1.vc) {
				newHistory.read = append(newHistory.read, k)
			}
		}

		newAccessEvent := accessEvent{VC: t1.vc.clone(), Lockset: lockset, Ev: p.Ev}

		for j := 0; j < len(oldHistory.write); j++ {
			if newAccessEvent.VC.less(oldHistory.write[j].VC) || oldHistory.write[j].VC.less(newAccessEvent.VC) {
				continue
			}
			jlock := oldHistory.write[j].Lockset
			if len(Intersection(newAccessEvent.Lockset, jlock)) == 0 {
				report.ReportRace(
					report.Location{File: oldHistory.write[j].Ev.Ops[0].SourceRef, Line: oldHistory.write[j].Ev.Ops[0].Line, W: true},
					report.Location{File: newAccessEvent.Ev.Ops[0].SourceRef, Line: newAccessEvent.Ev.Ops[0].Line, W: false}, false, 0)
			}
		}

		newHistory.read = append(newHistory.read, newAccessEvent)
	} else { //volatile synchronize
		vol, ok := volatiles[p.T2]
		if !ok {
			vol = newvc2()
		}
		t1.vc = t1.vc.ssync(vol)
		vol = t1.vc.clone()
		volatiles[p.T2] = vol
	}

	threads[p.T1] = t1
	variableMap[p.T2] = newHistory
}
