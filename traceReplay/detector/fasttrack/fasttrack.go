package fastTrack

import (
	"fmt"

	"../../util"
	"../analysis"
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
type ListenerSelect struct{}
type ListenerGoStart struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}
type ListenerNT struct{}
type ListenerNTWT struct{}
type ListenerPostProcess struct{}

type EventCollector struct{}

var listeners []traceReplay.EventListener

func Init() {
	threads = make(map[uint32]vcepoch)
	channels = make(map[uint32]channel)
	variables = make(map[uint32]variable)
	closedChans = make(map[uint32]vcepoch)
	signalList = make(map[uint32]vcepoch)
	locks = make(map[uint32]vcepoch)
	volatiles = make(map[uint32]vcepoch)
	notifies = make(map[uint32]vcepoch)

	listeners = []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerChanClose{},
		&ListenerOpClosedChan{},
		&ListenerSync{},
		&ListenerDataAccess2{},
		//	&ListenerSelect{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerNT{},
		&ListenerNTWT{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("fasttrack", &EventCollector{})
}

func (l *EventCollector) Put(p *util.SyncPair) {
	for _, l := range listeners {
		l.Put(p)
	}
}

var threads map[uint32]vcepoch
var channels map[uint32]channel
var variables map[uint32]variable
var closedChans map[uint32]vcepoch
var signalList map[uint32]vcepoch
var locks map[uint32]vcepoch
var volatiles map[uint32]vcepoch
var notifies map[uint32]vcepoch

type channel struct {
	buf   []vcepoch
	next  uint
	rnext uint
	count int
}

type variable struct {
	rvc    vcepoch
	wvc    vcepoch
	lastOp *util.Item
}

func newchan(bufsize uint32) channel {
	c := channel{make([]vcepoch, bufsize), 0, 0, 0}
	for i := range c.buf {
		c.buf[i] = newvc2()
	}

	return c
}
func newvar() variable {
	return variable{newEpoch(0, 0), newEpoch(0, 0), nil}
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
		lock = newvc2()
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newvc2().set(p.T1, 1)
	}

	t1 = t1.ssync(lock)

	threads[p.T1] = t1
	// if !p.AsyncSend {
	// 	return
	// }

	// if p.Lock {
	// 	handleLock2(p)
	// 	return
	// }

	// ch, ok := channels[p.T2]
	// if !ok {
	// 	bufsize := p.Ev.Ops[0].BufSize
	// 	ch = newchan(bufsize)
	// }

	// if ch.count == len(ch.buf) { // no free slot
	// 	return
	// }

	// t1VC, ok := threads[p.T1]
	// if !ok {
	// 	t1VC = newvc2()
	// }

	// t1VC.add(p.T1, 2)
	// t1VC = t1VC.sync(ch.buf[ch.next])
	// ch.buf[ch.next] = t1VC.clone()

	// ch.count++
	// ch.next = (ch.next + 1) % uint(len(ch.buf))

	// threads[p.T1] = t1VC
	// channels[p.T2] = ch
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
		lock = newvc2()
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newvc2().set(p.T1, 1)
	}

	lock = t1.clone()
	t1 = t1.add(p.T1, 1)

	threads[p.T1] = t1
	locks[p.T2] = lock
	// if !p.AsyncRcv {
	// 	return
	// }

	// if p.Unlock {
	// 	handleLock2(p)
	// 	return
	// }

	// ch, ok := channels[p.T2]
	// if !ok {
	// 	bufsize := p.Ev.Ops[0].BufSize
	// 	ch = newchan(bufsize)
	// }

	// if ch.count == 0 { //nothing to receive
	// 	return
	// }

	// t1VC, ok := threads[p.T1]
	// if !ok {
	// 	t1VC = newvc2()
	// }
	// t1VC.add(p.T1, 2)
	// t1VC = t1VC.sync(ch.buf[ch.rnext])
	// ch.buf[ch.rnext] = t1VC.clone()

	// ch.count--
	// ch.rnext = (ch.rnext + 1) % uint(len(ch.buf))

	// threads[p.T1] = t1VC
	// channels[p.T2] = ch
}

func (l *ListenerSync) Put(p *util.SyncPair) {
	if !p.Sync {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newvc2()
	}
	t2, ok2 := threads[p.T2]
	if !ok2 {
		t2 = newvc2()
	}

	//prepare + commit
	t1.add(p.T1, 2)
	t2.add(p.T2, 2)
	tmp := t1.sync(t2)
	t1 = tmp
	t2 = tmp.clone()

	threads[p.T1] = t1
	threads[p.T2] = t2
}

func handleLock2(p *util.SyncPair) {
	ev := p.Ev

	ch, ok := channels[p.T2]
	if !ok {
		bufsize := ev.Ops[0].BufSize
		ch = newchan(bufsize)
	}

	t1VC, ok := threads[p.T1]
	if !ok {
		t1VC = newvc2()
	}

	if ev.Ops[0].Mutex&util.LOCK > 0 || ev.Ops[0].Mutex&util.RLOCK > 0 {
		if ch.count > 0 {
			return
		}
		t1VC.add(p.T1, 2)
		t1VC = t1VC.sync(ch.buf[0])
		ch.buf[0] = t1VC.clone()
		ch.buf[1] = ch.buf[0]
		ch.count = 2
	} else if ev.Ops[0].Mutex&util.UNLOCK > 0 || ev.Ops[0].Mutex&util.RUNLOCK > 0 {
		if ch.count != 2 {
			return
		}
		t1VC.add(p.T1, 2)
		t1VC = t1VC.sync(ch.buf[0])
		ch.buf[0] = t1VC.clone()
		ch.buf[1] = ch.buf[0]
		ch.count = 0
	}

	threads[p.T1] = t1VC
	channels[p.T2] = ch
}

func (l *ListenerChanClose) Put(p *util.SyncPair) {
	if !p.DoClose {
		return
	}
	t1VC, ok := threads[p.T1]
	if !ok {
		t1VC = newvc2()
	}

	t1VC.add(p.T1, 2)

	closedChans[p.T2] = t1VC.clone()
	threads[p.T1] = t1VC
}

func (l *ListenerOpClosedChan) Put(p *util.SyncPair) {
	if !p.Closed {
		return
	}
	t1VC, ok := threads[p.T1]
	if !ok {
		t1VC = newvc2()
	}

	t1VC.add(p.T1, 2)
	cvc := closedChans[p.T2].clone()
	t1VC = t1VC.sync(cvc)

	threads[p.T1] = t1VC
}

var raceMap = make(map[report.Location]struct{})
var allRaces uint64
var uniqueRaces uint64

func (l *ListenerDataAccess2) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	ev := p.Ev

	t1VC, ok := threads[p.T1]
	if !ok {
		t1VC = newvc2().set(p.T1, 1)
	}
	t1VC = t1VC.add(p.T1, 1)

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newvar()
	}
	// fmt.Println("1", t1VC)
	// fmt.Println("2", varstate.wvc)
	// fmt.Println("3", varstate.rvc)
	//	isAtomic := ev.Ops[0].Kind&util.ATOMICREAD > 0 || ev.Ops[0].Kind&util.ATOMICWRITE > 0

	//BUG: x++ is a read + write, read detects race, replaces lastOp with the
	// read op, next is the write, detects the write-write race
	// but last op is set to the read of the same op, error report is nonsense
	// therefore. Update it similar to threadsanitizer with history for last op.
	if p.Write {
		if !varstate.wvc.leq(t1VC) {
			report.ReportRace(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
			// force order in the next step
		}
		varstate.wvc = newEpoch(p.T1, t1VC.get(p.T1))

		if !varstate.rvc.leq(t1VC) {
			report.ReportRace(report.Location{File: 1, Line: 1, W: false},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
		}
	} else if !p.Write {
		if !varstate.wvc.leq(t1VC) {
			report.ReportRace(report.Location{File: 1, Line: 1, W: true},
				report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: false}, false, 0)
		}
		if !varstate.rvc.leq(t1VC) {
			varstate.rvc = varstate.rvc.set(p.T1, t1VC.get(p.T1))
		} else {
			varstate.rvc = newEpoch(p.T1, t1VC.get(p.T1))
		}
	} else { //volatile synchronize
		vol, ok := volatiles[p.T2]
		if !ok {
			vol = newvc2()
		}
		t1VC = t1VC.ssync(vol)
		vol = t1VC.clone()
		volatiles[p.T2] = vol
	}

	varstate.lastOp = ev

	variables[p.T2] = varstate
	threads[p.T1] = t1VC

	// fmt.Println("4", t1VC)
	// fmt.Println("5", varstate.wvc)
	// fmt.Println("6", varstate.rvc)
	// fmt.Println("-----------")
}

func (l *ListenerGoFork) Put(p *util.SyncPair) {
	if !p.IsFork {
		return
	}
	t1VC, ok := threads[p.T1]
	if !ok {
		t1VC = newvc2().set(p.T1, 1)
	}

	signalList[p.T2] = t1VC.clone()

	t1VC = t1VC.add(p.T1, 1)

	threads[p.T1] = t1VC
}

func (l *ListenerGoWait) Put(p *util.SyncPair) {
	if !p.IsWait {
		return
	}

	vc, ok := signalList[p.T2]

	if ok {
		t1VC, ok := threads[p.T1]
		if !ok {
			t1VC = newvc2().set(p.T1, 1)
		}
		t1VC = t1VC.ssync(vc)
		t1VC = t1VC.add(p.T1, 1)
		threads[p.T1] = t1VC
	}
}

func (l *ListenerNT) Put(p *util.SyncPair) {
	if !p.IsNT {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newvc2().set(p.T1, 1)
	}
	vc, ok := notifies[p.T2]
	if !ok {
		vc = newvc2()
	}

	vc = vc.ssync(t1)
	t1 = t1.add(p.T1, 1)

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
			t1 = newvc2().set(p.T1, 1)
		}

		t1 = t1.ssync(vc)
		t1 = t1.add(p.T1, 1)
		vc = t1.clone()

		threads[p.T1] = t1
		notifies[p.T2] = vc
	}

}

func (l *ListenerPostProcess) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}
	fmt.Printf("Races FT (no Partner):%v/%v\n", uniqueRaces, allRaces)
}
