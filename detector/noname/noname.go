package noname

// import (
// 	"fmt"

// 	"../../util"
// 	"../analysis"
// 	"../graphbuilder"
// 	"../report"
// 	"../traceReplay"
// )

// type ListenerAsyncSnd struct{}
// type ListenerAsyncRcv struct{}
// type ListenerChanClose struct{}
// type ListenerOpClosedChan struct{}
// type ListenerSync struct{}
// type ListenerDataAccess struct{}
// type ListenerDataAccess2 struct{}
// type ListenerSelect struct{}
// type ListenerGoStart struct{}
// type ListenerGoFork struct{}
// type ListenerGoWait struct{}

// type EventCollector struct{}

// var listeners []traceReplay.EventListener

// func init() {
// 	listeners = []traceReplay.EventListener{
// 		&graphbuilder.ListenerGoFork{},
// 		&graphbuilder.ListenerGoWait{},
// 		&graphbuilder.ListenerSync{},
// 		&graphbuilder.ListenerAsyncSnd{},
// 		&graphbuilder.ListenerAsyncRcv{},
// 		&graphbuilder.ListenerDataAccess2{},
// 		&ListenerAsyncSnd{},
// 		&ListenerAsyncRcv{},
// 		&ListenerChanClose{},
// 		&ListenerOpClosedChan{},
// 		&ListenerSync{},
// 		&ListenerDataAccess2{},
// 		&ListenerSelect{},
// 		&ListenerGoFork{},
// 		&ListenerGoWait{},
// 	}
// 	algos.RegisterDetector("noname", &EventCollector{})
// }

// func (l *EventCollector) Put(m *traceReplay.Machine, p *util.SyncPair) {
// 	for _, l := range listeners {
// 		l.Put(m, p)
// 	}
// }

// var LastWrite map[uint64]*util.Item

// func init() {
// 	LastWrite = make(map[uint64]*util.Item)
// }

// func handleLock(m *traceReplay.Machine, p *util.SyncPair) {
// 	t1 := m.Threads[p.T1]
// 	ch := m.AsyncChans2[p.T2]
// 	ev := t1.Peek()

// 	if ev.Ops[0].Mutex&util.LOCK > 0 {
// 		if ch.Count > 0 {
// 			return
// 		}
// 		t1.VC.Add(t1.ID, 2)
// 		ch.Buf[0].Item = ev
// 		ch.Buf[1].Item = ev
// 		//t1.VC.Sync(ch.Buf[0].VC)
// 		//t1.VC.Sync(ch.Buf[1].VC)
// 		ch.Count = 2
// 	} else if ev.Ops[0].Mutex&util.UNLOCK > 0 {
// 		if ch.Count != 2 {
// 			return
// 		}
// 		t1.VC.Add(t1.ID, 2)
// 		//t1.VC.Sync(ch.Buf[0].VC)
// 		//t1.VC.Sync(ch.Buf[1].VC)
// 		ch.Buf[0].Item = nil
// 		ch.Buf[1].Item = nil
// 		ch.Count = 0
// 	} else if ev.Ops[0].Mutex&util.RLOCK > 0 {
// 		if ch.Count > 1 {
// 			return //writer lock already claimed both fields
// 		}
// 		t1.VC.Add(t1.ID, 2)
// 		//	t1.VC.Sync(ch.Buf[1].VC) //sync with writer vc
// 		ch.Buf[0].Item = nil
// 		ch.Rcounter++
// 		if ch.Count == 0 { //first reader
// 			ch.Count = 1
// 		}
// 	} else if ev.Ops[0].Mutex&util.RUNLOCK > 0 {
// 		if ch.Count > 1 {
// 			return // lock was a writer lock not a readlock
// 		}
// 		t1.VC.Add(t1.ID, 2)
// 		//	t1.VC.Sync(ch.Buf[0].VC) // sync with reader vc
// 		ch.Buf[0].Item = nil
// 		ch.Rcounter--
// 		if ch.Rcounter == 0 { //last reader
// 			ch.Count = 0
// 		}
// 	}

// 	t1.Pop()
// 	t1.Pop()
// 	m.Threads[p.T1] = t1
// 	m.AsyncChans2[p.T2] = ch
// }

// func (l *ListenerAsyncSnd) Put(m *traceReplay.Machine, p *util.SyncPair) {
// 	if !p.AsyncSend {
// 		return
// 	}

// 	t1 := m.Threads[p.T1]

// 	ev := t1.Peek()
// 	if ev.Ops[0].Mutex&util.LOCK > 0 || ev.Ops[0].Mutex&util.RLOCK > 0 {
// 		handleLock(m, p)
// 		return
// 	}

// 	ch := m.AsyncChans2[p.T2]
// 	if ch.Count == ch.BufSize { //check if there is a free slot
// 		return
// 	}
// 	if _, ok := m.ClosedChans[p.T2]; ok {
// 		fmt.Println("Send on closed channel!")
// 	}

// 	t1.VC.Add(t1.ID, 2) // update thread vc for successful storing (pre+post)
// 	//update bufferslot by storing the event and sync the vc of the thread and the bufferslot
// 	ch.Buf[ch.Next].Item = t1.Events[1]
// 	//t1.VC.Sync(ch.Buf[ch.Next].VC) //dont sync as a receiver with the buffer field?

// 	//only store the clock value of the thread that stored the message in the buffer VC!
// 	ch.Buf[ch.Next].VC = util.NewVC()
// 	ch.Buf[ch.Next].VC.Set(t1.ID, t1.VC[t1.ID])

// 	//TODO updating the state of the async chan should be handled by the chan itself not here
// 	ch.Next++ //next free slot
// 	if ch.Next >= ch.BufSize {
// 		ch.Next = 0
// 	}
// 	ch.Count++

// 	// cvc := m.ChanVC[p.t2]
// 	// cvc.Wvc.Sync(t1.VC.Clone())
// 	// m.ChanVC[p.t2] = cvc

// 	//remove the top event from the thread stack
// 	// ev.VC = t1.VC.Clone()
// 	// report.AddEvent(ev)

// 	t1.Pop() //pre
// 	t1.Pop() //post

// 	//update the traceReplay.Machine state
// 	m.Threads[t1.ID] = t1
// 	m.AsyncChans2[p.T2] = ch
// }
// func (l *ListenerAsyncRcv) Put(m *traceReplay.Machine, p *util.SyncPair) {
// 	if !p.AsyncRcv {
// 		return
// 	}
// 	t1 := m.Threads[p.T1]

// 	ev := t1.Peek()
// 	if ev.Ops[0].Mutex&util.UNLOCK > 0 || ev.Ops[0].Mutex&util.RUNLOCK > 0 {
// 		handleLock(m, p)
// 		return
// 	}

// 	ch := m.AsyncChans2[p.T2]
// 	if ch.Count > 0 { //is there something to receive?
// 		t1 := m.Threads[p.T1]
// 		t1.VC.Add(t1.ID, 2)             //update thread vc for succ rcv
// 		t1.VC.Sync(ch.Buf[ch.Rnext].VC) //sync thread vc and buffer slot

// 		//empty the buffer slot and update the chan state
// 		ch.Buf[ch.Rnext].Item = nil
// 		ch.Buf[ch.Rnext].VC = util.NewVC()

// 		ch.Rnext++ //next slot from which will be received
// 		if ch.Rnext >= ch.BufSize {
// 			ch.Rnext = 0
// 		}
// 		ch.Count--

// 		// ev.VC = t1.VC.Clone()
// 		// report.AddEvent(ev)
// 		t1.Pop() //pre
// 		t1.Pop() //post

// 		//update the traceReplay.Machine state
// 		m.Threads[t1.ID] = t1
// 		m.AsyncChans2[p.T2] = ch
// 	}
// }

// func (l *ListenerChanClose) Put(m *traceReplay.Machine, p *util.SyncPair) {
// 	if !p.DoClose {
// 		return
// 	}
// 	t1 := m.Threads[p.T1]

// 	cvc := m.ChanVC[p.T2]
// 	if cvc != nil && !cvc.Wvc.Less(t1.VC) {
// 		fmt.Println("Concurrent send during close operation on channel ", p.T2)
// 	}

// 	t1.VC.Add(t1.ID, 2)
// 	m.ClosedChansVC[p.T2] = t1.VC.Clone()
// 	m.ClosedChans[p.T2] = struct{}{}
// 	t1.Pop()
// 	t1.Pop()
// 	m.Threads[t1.ID] = t1
// }
// func (l *ListenerOpClosedChan) Put(m *traceReplay.Machine, p *util.SyncPair) {
// 	if !p.Closed {
// 		return
// 	}
// 	t1 := m.Threads[p.T1]

// 	if _, ok := m.ClosedChans[p.T2]; ok && t1.Events[0].Ops[0].Ch == p.T2 &&
// 		t1.Events[0].Ops[0].Kind&util.SEND > 0 {
// 		fmt.Println("Send on closed channel!")
// 	}

// 	t1.VC.Add(t1.ID, 2)
// 	// ev := t1.Peek()
// 	// ev.VC = t1.VC.Clone()
// 	// report.AddEvent(ev)
// 	t1.Pop()
// 	t1.Pop()
// 	m.Threads[t1.ID] = t1
// }
// func (l *ListenerSync) Put(m *traceReplay.Machine, p *util.SyncPair) {
// 	if !p.Sync {
// 		return
// 	}

// 	t1 := m.Threads[p.T1]
// 	t2 := m.Threads[p.T2]

// 	var ch uint64
// 	if len(t1.Events[0].Ops) == 1 {
// 		ch = t1.Events[0].Ops[0].Ch
// 	} else {
// 		ch = t2.Events[0].Ops[0].Ch
// 	}

// 	if _, ok := m.ClosedChans[ch]; ok {
// 		fmt.Println("Send on closed channel!")
// 	}
// 	// for _, s := range m.Selects {
// 	// 	for _, x := range s.Ev.Ops {
// 	// 		dual := util.OpKind(util.PREPARE | util.SEND)
// 	// 		if x.Kind == util.PREPARE|util.SEND {
// 	// 			dual = util.PREPARE | util.RCV
// 	// 		}
// 	// 		if x.Ch == ch && dual == t1.Events[0].Ops[0].Kind {
// 	// 			tmp := fmt.Sprintf("%v", s.Ev)
// 	// 			report.SelectAlternative(tmp, t1.Events[0].String())
// 	// 			//	fmt.Printf("2Event %v is an alternative for select statement \n\t%v\n", t1.Events[0], s.Ev)
// 	// 		} else if x.Ch == ch && dual == t2.Events[0].Ops[0].Kind {
// 	// 			tmp := fmt.Sprintf("%v", s.Ev)
// 	// 			tmp2 := fmt.Sprintf("%v", t2.Events[0])
// 	// 			report.SelectAlternative(tmp, tmp2)
// 	// 			//	fmt.Printf("3Event %v is an alternative for select statement \n\t%v\n", t2.Events[0], s.Ev)
// 	// 		}
// 	// 	}
// 	// }

// 	//prepare + commit
// 	t1.VC.Add(t1.ID, 2)
// 	t2.VC.Add(t2.ID, 2)
// 	// sync only the direct communication!
// 	t1.VC.Set(t2.ID, t2.VC[t2.ID])
// 	t2.VC.Set(t1.ID, t1.VC[t1.ID])
// 	//t1.VC.Sync(t2.VC) //sync updates both
// 	//	t2.VC.Sync(t1.VC)
// 	t1.Pop() //pre
// 	t2.Pop() //pre

// 	t1.Pop() //post
// 	t2.Pop() //post

// 	m.Threads[t1.ID] = t1
// 	m.Threads[t2.ID] = t2
// }

// func (l *ListenerDataAccess2) Put(m *traceReplay.Machine, p *util.SyncPair) {
// 	if !p.DataAccess {
// 		return
// 	}
// 	thread := m.Threads[p.T1]
// 	ev := thread.Peek()

// 	isAtomic := ev.Ops[0].Kind&util.ATOMICREAD > 0 || ev.Ops[0].Kind&util.ATOMICWRITE > 0

// 	thread.VC.Add(thread.ID, 1)
// 	//ev.VC = thread.VC.Clone()
// 	varstate := m.Vars3[ev.Ops[0].Ch]

// 	if varstate == nil {
// 		varstate = &util.VarState3{Rvc: util.NewVC(), Wvc: util.NewVC(),
// 			LastAccess: thread.ID, State: util.EXCLUSIVE,
// 			LastOp: &ev.Ops[0]}
// 	}

// 	//BUG: x++ is a read + write, read detects race, replaces lastOp with the
// 	// read op, next is the write, detects the write-write race
// 	// but last op is set to the read of the same op, error report is nonsense
// 	// therefore. Update it similar to threadsanitizer with history for last op.
// 	if ev.Ops[0].Kind&util.WRITE > 0 {
// 		LastWrite[p.T2] = ev
// 		if !varstate.Wvc.Less(thread.VC) {
// 			if !isAtomic {
// 				report.Race2(varstate.LastEv, ev, report.NORMAL)
// 			}
// 			// varstate.Wvc = util.NewVC()
// 			// varstate.Rvc = util.NewVC()
// 			varstate.Wvc.Set(thread.ID, thread.VC[thread.ID])
// 		} else {
// 			// varstate.Wvc = util.NewVC()
// 			varstate.Wvc.Set(thread.ID, thread.VC[thread.ID])
// 		}
// 		if !varstate.Rvc.Less(thread.VC) {
// 			if !isAtomic {
// 				report.Race2(varstate.LastEv, ev, report.NORMAL)
// 			}
// 			// varstate.Wvc = util.NewVC()
// 			// varstate.Rvc = util.NewVC()
// 		}
// 	} else if ev.Ops[0].Kind&util.READ > 0 {
// 		if !varstate.Wvc.Less(thread.VC) {
// 			if !isAtomic {
// 				report.Race2(varstate.LastEv, ev, report.NORMAL)
// 			}
// 			// varstate.Wvc = util.NewVC()
// 			// varstate.Rvc = util.NewVC()
// 		}
// 		if !varstate.Rvc.Less(thread.VC) {
// 			varstate.Rvc.Set(thread.ID, thread.VC[thread.ID])
// 		} else {
// 			varstate.Rvc = util.NewVC()
// 			varstate.Rvc.Set(thread.ID, thread.VC[thread.ID])
// 		}
// 		//w := LastWrite[p.T2]
// 		//thread.VC.Sync(w.VC.Clone())
// 	}

// 	varstate.LastAccess = thread.ID
// 	varstate.LastOp = &ev.Ops[0]
// 	varstate.LastEv = ev
// 	varstate.LastSP = p

// 	m.Vars3[ev.Ops[0].Ch] = varstate
// 	thread.Pop()
// 	m.Threads[p.T1] = thread
// }

// func (l *ListenerSelect) Put(m *traceReplay.Machine, p *util.SyncPair) {
// 	if !p.IsSelect {
// 		return
// 	}

// 	threadSelect := m.Threads[p.T1]
// 	//	if p.closed { //default case

// 	vc := threadSelect.VC.Clone()
// 	vc.Add(p.T1, 1)
// 	it := threadSelect.Events[0]
// 	m.Selects = append(m.Selects, traceReplay.SelectStore{vc, it})
// 	//} else { //chan op
// 	//	fmt.Println("2>>>", p)
// 	//	fmt.Println("SELECT")
// 	//}
// }

// func (l *ListenerGoStart) Put(m *traceReplay.Machine, p *util.SyncPair) {
// 	if !p.IsGoStart {
// 		return
// 	}

// 	t1 := m.Threads[p.T1]
// 	t2 := m.Threads[p.T2]
// 	t1.VC.Add(t1.ID, 1)
// 	t2.VC.Sync(t1.VC)
// 	t2.VC.Add(t2.ID, 1)

// 	t1.Pop()
// 	t2.Pop()
// 	m.Threads[p.T1] = t1
// 	m.Threads[p.T2] = t2
// }

// func (l *ListenerGoFork) Put(m *traceReplay.Machine, p *util.SyncPair) {
// 	if !p.IsFork {
// 		return
// 	}
// 	t1 := m.Threads[p.T1]
// 	t1.VC.Add(t1.ID, 1)

// 	ev := t1.Peek().Clone()
// 	m.SignalList = append(m.SignalList, ev)

// 	t1.Pop()
// 	m.Threads[p.T1] = t1
// }

// func (l *ListenerGoWait) Put(m *traceReplay.Machine, p *util.SyncPair) {
// 	if !p.IsWait {
// 		return
// 	}

// 	t1 := m.Threads[p.T1]
// 	ev := t1.Peek()

// 	for i := range m.SignalList {
// 		if m.SignalList[i].Ops[0].Ch == ev.Ops[0].Ch {
// 			// found fitting signal for this wait
// 			//t1.VC.Sync(m.SignalList[i].VC)
// 			t1.VC.Add(t1.ID, 1)
// 			t1.Pop()
// 			m.Threads[p.T1] = t1
// 			return
// 		}
// 	}
// }
