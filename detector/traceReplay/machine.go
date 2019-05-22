package traceReplay

import (
	"../../util"
)

type EventListener interface {
	Put(*util.SyncPair)
}

var EvListener = []EventListener{}

type Stepper struct{}

func (s *Stepper) Put(m *Machine, p *util.SyncPair) {
	t1 := m.Threads[p.T1]

	if p.AsyncSend {
		ch := m.AsyncChans2[p.T2]
		if ch.Count == ch.BufSize {
			return
		}

		ev := t1.Peek()

		if ev.Ops[0].Mutex&util.LOCK > 0 || ev.Ops[0].Mutex&util.RLOCK > 0 {
			if ch.Count > 0 {
				return
			}
			ch.Buf[0].Item = ev
			ch.Buf[1].Item = ev
			ch.Count = 2
		} else {
			ch.Buf[ch.Next].Item = t1.Events[1]

			ch.Next = (ch.Next + 1) % ch.BufSize
			ch.Count++
		}

		m.AsyncChans2[p.T2] = ch
	} else if p.AsyncRcv {
		ch := m.AsyncChans2[p.T2]
		if ch.Count == 0 {
			return
		}

		ev := t1.Peek()

		if ev.Ops[0].Mutex&util.UNLOCK > 0 || ev.Ops[0].Mutex&util.RUNLOCK > 0 {
			ch.Buf[0].Item = nil
			ch.Buf[1].Item = nil
			ch.Count = 0
		} else {
			ch.Buf[ch.Rnext].Item = nil
			ch.Rnext = (ch.Rnext + 1) % ch.BufSize
		}

		m.AsyncChans2[p.T2] = ch
	} else if p.DoClose {
		m.ClosedChans[p.T2] = struct{}{}
	} else if p.Sync {
		t2 := m.Threads[p.T2]
		t2.Pop()
		t2.Pop()
		m.Threads[t2.ID] = t2
	} else if p.IsFork {
		//	m.SignalList = append(m.SignalList, t1.Peek().Clone())
		ev := t1.Peek().Clone()
		m.SignalList2[ev.Ops[0].Ch] = ev
	} else if p.IsWait {
		ev := t1.Peek()

		_, ok := m.SignalList2[ev.Ops[0].Ch]

		if !ok {
			return
		}
	}

	t1.Pop()
	// if !p.DataAccess && !p.IsFork && !p.IsWait {
	// 	t1.Pop()
	// }

	m.Threads[t1.ID] = t1
}

type Machine struct {
	Threads       map[uint32]util.Thread
	AsyncChans    map[uint32][]*util.Item
	ClosedChans   map[uint32]struct{}
	ClosedChansVC map[uint32]util.VectorClock
	Stopped       bool
	Vars1         map[uint32]*util.VarState1
	Vars2         map[uint32]*util.VarState2
	Vars3         map[uint32]*util.VarState3
	AsyncChans2   map[uint32]util.AsyncChan
	ChanVC        map[uint32]*util.ChanState
	Selects       []SelectStore
	AccessCounter uint64
	FieldCounter  uint32
	LockCounter   uint32
	SignalList    []*util.Item
	SignalList2   map[uint32]*util.Item
}

type SelectStore struct {
	VC util.VectorClock
	Ev *util.Item
}

func (m Machine) Clone() Machine {
	threads := make(map[uint32]util.Thread)
	for k, v := range m.Threads {
		threads[k] = v.Clone()
	}
	asyncChans := make(map[uint32][]*util.Item)
	for k, v := range m.AsyncChans {
		var ops []*util.Item
		for _, i := range v {
			ops = append(ops, i.Clone())
		}
		asyncChans[k] = ops
	}
	closedChans := make(map[uint32]struct{})
	for k, v := range m.ClosedChans {
		closedChans[k] = v
	}
	return Machine{Threads: threads, AsyncChans: asyncChans, ClosedChans: closedChans, ClosedChansVC: nil, Stopped: false}
}

func (m *Machine) GetSyncPairs() (ret []util.SyncPair) {
	for _, t := range m.Threads {
		if len(t.Events) == 0 {
			continue
		}
		e := t.Peek()
		for i, op := range e.Ops {
			if op.Kind&util.CLS > 0 {
				ret = append(ret, util.SyncPair{T1: t.ID, T2: op.Ch, DoClose: true, Idx: i})
				continue
			}
			if op.Kind&util.RCV == 0 || op.BufSize > 0 {
				continue
			}
			if _, ok := m.ClosedChans[op.Ch]; ok {
				//closed channel
				ret = append(ret, util.SyncPair{T1: t.ID, T2: op.Ch, Closed: true, Idx: i})
				continue
			}
			for _, p := range m.Threads {
				if p.ID == t.ID || len(p.Events) == 0 {
					continue
				}
				pe := p.Peek()
				for j, pop := range pe.Ops {
					if pop.Ch == op.Ch && pop.Kind == util.OpKind(util.PREPARE|util.SEND) {
						ret = append(ret, util.SyncPair{T1: t.ID, T2: p.ID, Idx: i, T2Idx: j})
					}
				}
			}

		}
	}
	return
}

func (m *Machine) GetAsyncActions() (ret []util.SyncPair) {
	for _, t := range m.Threads {
		if len(t.Events) == 0 {
			continue
		}
		e := t.Peek()

		for _, op := range e.Ops {
			if op.Kind == util.OpKind(util.PREPARE|util.SEND) && op.BufSize > 0 {
				c := m.AsyncChans2[op.Ch]
				if c.Count <= c.BufSize {
					ret = append(ret, util.SyncPair{T1: t.ID, T2: op.Ch, AsyncSend: true})
				}
			} else if op.Kind == util.OpKind(util.PREPARE|util.RCV) && op.BufSize > 0 {
				c := m.AsyncChans2[op.Ch]
				if c.Count > 0 {
					ret = append(ret, util.SyncPair{T1: t.ID, T2: op.Ch, AsyncRcv: true})
				}
			}
		}
	}
	return
}

func (m *Machine) GetThreadStarts() (ret map[uint32]uint32) {
	ret = make(map[uint32]uint32)
	for _, t := range m.Threads {
		if len(t.Events) == 0 {
			continue
		}
		e := t.Peek()

		for _, op := range e.Ops {
			partner := util.OpKind(util.SIG)
			if op.Kind&util.SIG > 0 {
				partner = util.WAIT
			} else if op.Kind&util.WAIT > 0 {
				partner = util.SIG
			} else {
				break
			}
			for _, j := range m.Threads {
				if len(j.Events) == 0 || j.ID == t.ID {
					continue
				}
				for _, op2 := range j.Peek().Ops {
					if op2.Kind&partner > 0 && op.Ch == op2.Ch {
						if op.Kind&util.SIG > 0 {
							ret[t.ID] = j.ID
							//ret = append(ret, SyncPair{t1: t.ID, t2: j.ID, GoStart: true})
						} else {
							ret[j.ID] = t.ID
							//	ret = append(ret, SyncPair{t1: j.ID, t2: t.ID, GoStart: true})
						}
					}
				}
			}
		}
	}
	return
}

// func (m *Machine) StartAllthreads() {
// 	for {
// 		threads := m.GetThreadStarts()
// 		if len(threads) == 0 {
// 			return
// 		}

// 		for k, v := range threads {
// 			t1 := m.Threads[k]
// 			t2 := m.Threads[v]
// 			t1.VC.Add(t1.ID, 1)
// 			t2.VC.Sync(t1.VC)
// 			t2.VC.Add(t2.ID, 1) //?
// 			t1.Pop()
// 			t2.Pop()
// 			m.Threads[k] = t1
// 			m.Threads[v] = t2
// 		}
// 	}
// }

func (m *Machine) ClosedRcv(p util.SyncPair, rcvOnClosed map[string]struct{}) {
	if p.Closed {
		t := m.Threads[p.T1]
		if p.T2 != 0 {
			rcvOnClosed[t.Peek().String()] = struct{}{}
		}

		t.Pop()
		t.Pop()
		m.Threads[p.T1] = t
	}
}

func (m *Machine) CloseChan(p util.SyncPair) {
	t := m.Threads[p.T1]
	t.Pop()
	t.Pop()
	m.Threads[p.T1] = t
	m.ClosedChans[p.T2] = struct{}{}
}

//IMPORTANT NEW PARTS
func (m *Machine) GetNextAction() (ret []util.SyncPair) {
	var async []util.SyncPair
	var sync []util.SyncPair
	var dataAccess []util.SyncPair
	var close []util.SyncPair
	for _, t := range m.Threads {
		if len(t.Events) == 0 {
			continue
		}
		e := t.Peek()

		for i, op := range e.Ops {
			if op.BufSize > 0 && len(async) == 0 { //async op
				if op.Kind == util.PREPARE|util.SEND {
					c := m.AsyncChans2[op.Ch]
					if c.Count < c.BufSize {
						async = append(async, util.SyncPair{T1: t.ID, T2: op.Ch, AsyncSend: true})
					}
				} else if op.Kind == util.PREPARE|util.RCV {
					c := m.AsyncChans2[op.Ch]
					if c.Count > 0 {
						async = append(async, util.SyncPair{T1: t.ID, T2: op.Ch, AsyncRcv: true})
					}
				}
			} else if (op.Kind&util.WRITE > 0 || op.Kind&util.READ > 0) && len(dataAccess) == 0 { //data access
				dataAccess = append(dataAccess, util.SyncPair{T1: t.ID, DataAccess: true, Idx: i})
			} else if op.Kind&util.CLS > 0 && len(close) == 0 { //channel closing
				close = append(close, util.SyncPair{T1: t.ID, T2: op.Ch, DoClose: true, Idx: i})
			} else if op.Kind&util.RCV > 0 {
				for _, p := range m.Threads {
					if p.ID == t.ID || len(p.Events) == 0 {
						continue
					}
					pe := p.Peek()
					for j, pop := range pe.Ops {
						if pop.Ch == op.Ch && pop.Kind == util.PREPARE|util.SEND {
							sync = append(sync, util.SyncPair{T1: t.ID, T2: p.ID, Idx: i, T2Idx: j, Sync: true})
							return sync
						}
					}
				}
			} else if _, ok := m.ClosedChans[op.Ch]; ok && len(close) == 0 { //operation on a closed channel
				close = append(close, util.SyncPair{T1: t.ID, T2: op.Ch, Closed: true, Idx: i})
			}
		}
	}
	if len(async) > 0 {
		return async
	} else if len(close) > 0 {
		return close
	}
	return dataAccess
}

//GetNextActionWCommLink determines the next event with priority to sync operations. Memory accesses are delayed on purpose
func (m *Machine) GetNextActionWCommLink() (ret []util.SyncPair) {
	var async []util.SyncPair
	var sync []util.SyncPair
	var dataAccess []util.SyncPair
	var close []util.SyncPair
	for _, t := range m.Threads {
		if len(t.Events) == 0 {
			continue
		}
		e := t.Peek() //pre

		for i, op := range e.Ops {
			if op.Kind&util.CLS > 0 && len(close) == 0 {
				close = append(close, util.SyncPair{T1: t.ID, T2: op.Ch, DoClose: true, Idx: i})
			} else if op.BufSize > 0 && len(async) == 0 { //async op
				if op.Kind == util.PREPARE|util.SEND {
					c := m.AsyncChans2[op.Ch]
					if op.Mutex == util.RLOCK {
						// The lock must either be empty (c.Count < c.BufSize) or the current holder must be a reader
						// writer handling is sufficient with the empty lock conditions since writers are only allowed
						// if nobody else is using the lock
						if c.Count < 2 {
							async = append(async, util.SyncPair{T1: t.ID, T2: op.Ch, AsyncSend: true, IsSelect: len(e.Ops) > 1})
						}
					} else if op.Mutex == util.LOCK {
						if c.Count == 0 {
							async = append(async, util.SyncPair{T1: t.ID, T2: op.Ch, AsyncSend: true, IsSelect: len(e.Ops) > 1})
						}
					} else if c.Count < c.BufSize {
						async = append(async, util.SyncPair{T1: t.ID, T2: op.Ch, AsyncSend: true, IsSelect: len(e.Ops) > 1})
					}

				} else if op.Kind == util.PREPARE|util.RCV {
					c := m.AsyncChans2[op.Ch]
					if op.Mutex == util.RUNLOCK {
						if c.Count < 2 {
							async = append(async, util.SyncPair{T1: t.ID, T2: op.Ch, AsyncRcv: true, IsSelect: len(e.Ops) > 1})
						}
					} else if op.Mutex == util.UNLOCK {
						if c.Count == 2 {
							async = append(async, util.SyncPair{T1: t.ID, T2: op.Ch, AsyncRcv: true, IsSelect: len(e.Ops) > 1})
						}
					} else if c.Count > 0 {
						async = append(async, util.SyncPair{T1: t.ID, T2: op.Ch, AsyncRcv: true, IsSelect: len(e.Ops) > 1})
					}
					// if c.Count > 0 {
					// 	async = append(async, util.SyncPair{T1: t.ID, T2: op.Ch, AsyncRcv: true, IsSelect: len(e.Ops) > 1})
					// }
				}
			} else if (op.Kind&util.WRITE > 0 || op.Kind&util.READ > 0) && len(dataAccess) == 0 { //data access
				dataAccess = append(dataAccess, util.SyncPair{T1: t.ID, T2: op.Ch, DataAccess: true, Idx: i})
			} else if op.Kind&util.CLS > 0 && len(close) == 0 { //channel closing
				close = append(close, util.SyncPair{T1: t.ID, T2: op.Ch, DoClose: true, Idx: i})
			} else if _, ok := m.ClosedChans[op.Ch]; ok {
				close = append(close, util.SyncPair{T1: t.ID, T2: op.Ch, Closed: true, Idx: i, IsSelect: len(e.Ops) > 1})
			} else if op.Kind&util.RCV > 0 {
				if len(t.Events) == 1 {
					// pending rcv, executed during the post processing
					continue
				}
				partner := t.Events[1].Partner

				for _, p := range m.Threads {
					if p.ID == t.ID || len(p.Events) == 0 || partner != p.ID {
						continue
					}
					pe := p.Peek()
					for j, pop := range pe.Ops {
						if pop.Ch == op.Ch && pop.Kind == util.PREPARE|util.SEND {
							sync = append(sync, util.SyncPair{T1: t.ID, T2: p.ID, Idx: i, T2Idx: j, Sync: true, IsSelect: len(e.Ops) > 1})
							return sync
						}
					}
				}
			} else if _, ok := m.ClosedChans[op.Ch]; ok && len(close) == 0 { //operation on a closed channel
				close = append(close, util.SyncPair{T1: t.ID, T2: op.Ch, Closed: true, Idx: i, IsSelect: len(e.Ops) > 0})
			}
		}
	}
	if len(async) > 0 {
		return async
	} else if len(close) > 0 {
		return close
	}
	return dataAccess
}

func (m *Machine) GetNextRandomActionWCommLink() (ret []util.SyncPair) {
	for _, t := range m.Threads {
		if len(t.Events) == 0 {
			continue
		}
		e := t.Peek() //pre

		for i, op := range e.Ops {
			if op.Kind&util.CLS > 0 {
				ret = append(ret, util.SyncPair{T1: t.ID, T2: op.Ch,
					DoClose: true, Idx: i})
			} else if op.Kind&util.WRITE > 0 || op.Kind&util.READ > 0 {
				ret = append(ret, util.SyncPair{T1: t.ID, T2: op.Ch, DataAccess: true,
					Idx: i})
			} else if op.BufSize > 0 {
				if op.Kind == util.PREPARE|util.SEND {
					c := m.AsyncChans2[op.Ch]
					if c.Count < c.BufSize {
						ret = append(ret, util.SyncPair{T1: t.ID, T2: op.Ch,
							AsyncSend: true, IsSelect: len(e.Ops) > 1, Idx: i})
					}
				} else if op.Kind == util.PREPARE|util.RCV {
					c := m.AsyncChans2[op.Ch]
					if c.Count > 0 {
						ret = append(ret, util.SyncPair{T1: t.ID, T2: op.Ch,
							AsyncRcv: true, IsSelect: len(e.Ops) > 1, Idx: i})
					}
				}
			} else if _, ok := m.ClosedChans[op.Ch]; ok {
				ret = append(ret, util.SyncPair{T1: t.ID, T2: op.Ch, Closed: true,
					IsSelect: len(e.Ops) > 1, Idx: i})
			} else if op.Kind&util.RCV > 0 {
				if len(t.Events) == 1 {
					// pending rcv, executed during the post processing
					continue
				}
				partner := t.Events[1].Partner

				for _, p := range m.Threads {
					if p.ID == t.ID || len(p.Events) == 0 || partner != p.ID {
						continue
					}
					pe := p.Peek()
					for j, pop := range pe.Ops {
						if pop.Ch == op.Ch && pop.Kind == util.PREPARE|util.SEND {
							ret = append(ret, util.SyncPair{T1: t.ID, T2: p.ID,
								Idx: i, T2Idx: j, Sync: true, IsSelect: len(e.Ops) > 1})
						}
					}
				}
			}
			if len(ret) > 0 {
				return
			}
		}
	}

	return
}

// //only necessary for the search for alternative communication. uninteresting for this project.
// func (m *Machine) UpdateChanVc(p *util.SyncPair) {
// 	//	fmt.Println(">>", p.String())
// 	thread1 := m.Threads[p.T1]

// 	if p.AsyncSend {
// 		chanState := m.ChanVC[p.T2]
// 		if chanState == nil {
// 			chanState = util.NewChanState()
// 		}

// 		//prevReadCtxt := chanState.RContext

// 		if chanState.Wvc.Less(thread1.VC) {
// 			nVC := util.NewVC()
// 			nVC.AddEpoch(util.NewEpoch(thread1.ID, thread1.VC[thread1.ID]))
// 			chanState.Wvc = nVC
// 			//chanState.WContext = []string{thread1.ShortString()}
// 			chanState.WContext = make(map[uint64]string)
// 			chanState.WContext[thread1.ID] = thread1.ShortString()
// 		} else {
// 			chanState.Wvc.AddEpoch(util.NewEpoch(thread1.ID, thread1.VC[thread1.ID]))
// 			//chanState.WContext = append(chanState.WContext, thread1.ShortString())
// 			chanState.WContext[thread1.ID] = thread1.ShortString()
// 		}

// 		if len(chanState.Wvc) > 1 {
// 			s := fmt.Sprintf("%v", thread1.ShortString())
// 			//s = string(s[4 : len(s)-3])
// 			for _, v := range chanState.WContext {
// 				report.Alternative(s, v)
// 				//	fmt.Println(v)
// 			}
// 		}
// 		m.ChanVC[p.T2] = chanState

// 	} else if p.AsyncRcv {
// 		chanState := m.ChanVC[p.T2]
// 		if chanState == nil {
// 			chanState = util.NewChanState()
// 		}

// 		if chanState.Rvc.Less(thread1.VC) {
// 			nVC := util.NewVC()
// 			nVC.AddEpoch(util.NewEpoch(thread1.ID, thread1.VC[thread1.ID]))
// 			chanState.Rvc = nVC
// 			//chanState.RContext = []string{thread1.ShortString()}
// 			chanState.RContext = make(map[uint64]string)
// 			chanState.RContext[thread1.ID] = thread1.ShortString()
// 		} else {
// 			chanState.Rvc.AddEpoch(util.NewEpoch(thread1.ID, thread1.VC[thread1.ID]))
// 			//chanState.RContext = append(chanState.RContext, thread1.ShortString())
// 			chanState.RContext[thread1.ID] = thread1.ShortString()
// 		}

// 		if len(chanState.Rvc) > 1 {
// 			s := fmt.Sprintf("%v", thread1.ShortString())
// 			for _, v := range chanState.RContext {
// 				report.Alternative(s, v)
// 			}
// 		}

// 		m.ChanVC[p.T2] = chanState

// 	} else if p.Sync {
// 		thread2 := m.Threads[p.T2]
// 		chanState := m.ChanVC[thread1.Events[0].Ops[0].Ch]
// 		if chanState == nil {
// 			chanState = util.NewChanState()
// 		}
// 		//prevReadCtxt := chanState.RContext
// 		//prevWriteCtxt := chanState.WContext
// 		if chanState.Rvc.Less(thread1.VC) {
// 			nVC := util.NewVC()
// 			nVC.AddEpoch(util.NewEpoch(thread1.ID, thread1.VC[thread1.ID]))
// 			chanState.Rvc = nVC
// 			//	prevReadCtxt = chanState.RContext
// 			//chanState.RContext = []string{thread1.ShortString()}
// 			chanState.RContext = make(map[uint64]string)
// 			chanState.RContext[thread1.ID] = thread1.ShortString()
// 		} else {
// 			chanState.Rvc.AddEpoch(util.NewEpoch(thread1.ID, thread1.VC[thread1.ID]))
// 			//chanState.RContext = append(chanState.RContext, thread1.ShortString())
// 			chanState.RContext[thread1.ID] = thread1.ShortString()
// 		}

// 		if len(chanState.Rvc) > 1 {
// 			s := fmt.Sprintf("%v", thread1.ShortString())
// 			for _, v := range chanState.RContext {

// 				report.Alternative(s, v)
// 			}
// 		}

// 		if chanState.Wvc.Less(thread2.VC) {
// 			nVC := util.NewVC()
// 			nVC.AddEpoch(util.NewEpoch(thread2.ID, thread2.VC[thread2.ID]))
// 			chanState.Wvc = nVC
// 			//prevWriteCtxt = chanState.WContext
// 			//chanState.WContext = []string{thread2.ShortString()}
// 			chanState.WContext = make(map[uint64]string)
// 			chanState.WContext[thread2.ID] = thread2.ShortString()
// 		} else {
// 			chanState.Wvc.AddEpoch(util.NewEpoch(thread2.ID, thread2.VC[thread2.ID]))
// 			//chanState.WContext = append(chanState.WContext, thread2.ShortString())
// 			chanState.WContext[thread2.ID] = thread2.ShortString()
// 		}

// 		if len(chanState.Wvc) > 1 {
// 			s := fmt.Sprintf("%v", thread2.ShortString())
// 			for _, v := range chanState.WContext {

// 				report.Alternative(s, v)
// 			}
// 		}

// 		m.ChanVC[thread1.Events[0].Ops[0].Ch] = chanState
// 	} else if p.Closed && p.T2 == 0 {
// 		//default case of select

// 	}

// 	// fmt.Println("------------------------")
// 	// for k, v := range m.ChanVC {
// 	// 	fmt.Println(k)
// 	// 	fmt.Println(v.Wvc)
// 	// 	fmt.Println(v.Rvc)
// 	// 	fmt.Println(v.WContext, "||", v.RContext)
// 	// 	fmt.Println("%%%%%%%%%%%%%%%%%%%%%%%%%")
// 	// }
// }

func (m *Machine) HandleSigNWait() (sigs, waits []util.SyncPair) {
	for _, t := range m.Threads {
		if len(t.Events) == 0 {
			continue
		}

		e := t.Peek()

		//only signal and wait so ops should be size 1
		op := e.Ops[0]

		if op.Kind&util.SIG > 0 {
			sigs = append(sigs, util.SyncPair{T1: t.ID, T2: e.Ops[0].Ch, IsFork: true})
		} else if op.Kind&util.WAIT > 0 {
			waits = append(waits, util.SyncPair{T1: t.ID, T2: e.Ops[0].Ch, IsWait: true})
		}
	}
	return
}

func (m *Machine) GetNextActionv2() []util.SyncPair {
	for _, t := range m.Threads {
		if len(t.Events) == 0 {
			continue
		}
		e := t.Peek()

		// only for select stmts more then one operation
		for i, op := range e.Ops {
			//data access case
			if op.Kind&util.WRITE > 0 || op.Kind&util.READ > 0 {
				if op.PPC == m.AccessCounter {
					m.AllowNextAccess()
					return []util.SyncPair{util.SyncPair{T1: t.ID, T2: op.Ch, DataAccess: true, Idx: i}}
				}
				continue
			}
			if op.Kind&util.ATOMICWRITE > 0 || op.Kind&util.ATOMICREAD > 0 {
				if op.PPC == m.AccessCounter {
					m.AllowNextAccess()
					return []util.SyncPair{util.SyncPair{T1: t.ID, T2: op.Ch, DataAccess: true, Idx: i}}
				}
				continue
			}

			if op.Kind&util.SIG > 0 {

				//m.SignalList2[op.Ch] = e
				return []util.SyncPair{util.SyncPair{T1: t.ID, T2: e.Ops[0].Ch, IsFork: true}}
			} else if op.Kind&util.WAIT > 0 {
				_, ok := m.SignalList2[op.Ch]
				if ok {
					return []util.SyncPair{util.SyncPair{T1: t.ID, T2: e.Ops[0].Ch, IsWait: true}}
				}
			}

			// buffered channel case
			if op.BufSize > 0 {
				c := m.AsyncChans2[op.Ch]

				if op.Mutex == util.LOCK { // mutex.Lock()

					if c.Count == 0 && op.BufField == m.LockCounter { //is lockable, must be 0 due to rwlock support
						m.LockCounter++
						return []util.SyncPair{util.SyncPair{T1: t.ID, T2: op.Ch, AsyncSend: true, IsSelect: len(e.Ops) > 1}}
					}
				} else if op.Mutex == util.UNLOCK {
					return []util.SyncPair{util.SyncPair{T1: t.ID, T2: op.Ch, AsyncRcv: true, IsSelect: len(e.Ops) > 1}}
				}

				switch op.Kind {
				case util.PREPARE | util.SEND:

					if len(t.Events) < 2 { // for dangling pres
						continue
					}

					postBufField := t.Events[1].Ops[0].BufField

					if c.Count < c.BufSize && postBufField == c.SndCounter {
						c.SndCounter++
						m.AsyncChans2[op.Ch] = c
						return []util.SyncPair{util.SyncPair{T1: t.ID, T2: op.Ch, AsyncSend: true, IsSelect: len(e.Ops) > 1}}
					}

				case util.PREPARE | util.RCV:

					if len(t.Events) == 1 {
						continue
					}

					if c.Count > 0 { //is something stored in the buffer?

						//we need to check if its from the right sender
						//get next read item from the buffer of c
						it := c.Buf[c.Rnext].Item
						partner := t.Events[1].Partner
						if partner == it.Thread { //is it the right partner thread?
							// is it the right send from the partner thread?
							// loop because it could be a select statement
							for _, op2 := range it.Ops {
								if op2.PC == t.Events[1].Ops[0].PPC {
									return []util.SyncPair{util.SyncPair{T1: t.ID, T2: op.Ch, AsyncRcv: true, IsSelect: len(e.Ops) > 1}}
								}
							}
						}
					}
				}
				continue
			}

			// sync channel case
			if op.Kind&util.RCV > 0 { //rcver determines the next sender
				if len(t.Events) == 1 {
					continue
				}
				partner := t.Events[1].Partner
				ppc := t.Events[1].Ops[0].PPC

				for _, p := range m.Threads {
					if p.ID == t.ID || len(p.Events) == 0 || partner != p.ID {
						continue
					}
					pe := p.Peek()
					for j, pop := range pe.Ops {
						if pop.Ch == op.Ch && pop.Kind == util.PREPARE|util.SEND && ppc == pop.PC {
							state := m.ChanVC[op.Ch]
							if state == nil {
								state = util.NewChanState()
							}
							state.SndCnt++
							m.ChanVC[op.Ch] = state
							return []util.SyncPair{util.SyncPair{T1: t.ID, T2: p.ID, Idx: i, T2Idx: j, Sync: true, IsSelect: len(e.Ops) > 1}}
						}
					}
				}
			}

			if op.Kind&util.CLS > 0 {
				//check if its time for the close or not
				state := m.ChanVC[op.Ch]
				if state == nil {
					state = util.NewChanState()
				}
				if state.SndCnt == op.BufField {
					state.SndCnt++
					m.ChanVC[op.Ch] = state
					return []util.SyncPair{util.SyncPair{T1: t.ID, T2: op.Ch, DoClose: true, Idx: i}}
				}

			}
		}
	}

	return nil
}

func (m *Machine) GetNextActionv3() []util.SyncPair {
	for _, t := range m.Threads {
		if len(t.Events) == 0 {
			continue
		}
		e := t.Peek()

		// only for select stmts more then one operation
		for i, op := range e.Ops {
			//data access case
			if op.Kind&util.WRITE > 0 || op.Kind&util.READ > 0 {
				//	fmt.Printf("\nNORMAL %v | %v\n", op.PPC, m.AccessCounter)
				//	if op.PPC == m.AccessCounter {
				//	m.AllowNextAccess()
				return []util.SyncPair{util.SyncPair{T1: e.Thread, T2: op.Ch, DataAccess: true, Idx: i}}
				//	}
				//	continue
			}
			if op.Kind&util.ATOMICWRITE > 0 || op.Kind&util.ATOMICREAD > 0 {
				//	fmt.Printf("\nATOMIC %v | %v\n", op.PPC, m.AccessCounter)
				//	if op.PPC == m.AccessCounter {
				//		m.AllowNextAccess()
				return []util.SyncPair{util.SyncPair{T1: e.Thread, T2: op.Ch, DataAccess: true, Idx: i}}
				//	}
				//	continue
			}

			if op.Kind&util.SIG > 0 {
				return []util.SyncPair{util.SyncPair{T1: e.Thread, T2: e.Ops[0].Ch, IsFork: true}}
			} else if op.Kind&util.WAIT > 0 {
				_, ok := m.SignalList2[op.Ch]
				if ok {
					return []util.SyncPair{util.SyncPair{T1: e.Thread, T2: e.Ops[0].Ch, IsWait: true}}
				}
			}

			// buffered channel case
			if op.BufSize > 0 {
				c := m.AsyncChans2[op.Ch]

				switch op.Kind {
				case util.PREPARE | util.SEND:
					if op.Mutex == util.LOCK { // mutex.Lock()

						if c.Count == 0 && op.BufField == m.LockCounter { //is lockable, must be 0 due to rwlock support
							m.LockCounter++
							return []util.SyncPair{util.SyncPair{T1: e.Thread, T2: op.Ch, AsyncSend: true, IsSelect: len(e.Ops) > 1}}
						}
					}

					if len(t.Events) < 2 { // for dangling pres
						continue
					}

					postBufField := t.Events[1].Ops[0].BufField

					if c.Count < c.BufSize && postBufField == c.SndCounter {
						c.SndCounter++
						m.AsyncChans2[op.Ch] = c
						return []util.SyncPair{util.SyncPair{T1: e.Thread, T2: op.Ch, AsyncSend: true, IsSelect: len(e.Ops) > 1}}
					}

				case util.PREPARE | util.RCV:
					if op.Mutex == util.UNLOCK {
						return []util.SyncPair{util.SyncPair{T1: e.Thread, T2: op.Ch, AsyncRcv: true, IsSelect: len(e.Ops) > 1}}
					}

					if len(t.Events) == 1 {
						continue
					}

					if c.Count > 0 { //is something stored in the buffer?

						//we need to check if its from the right sender
						//get next read item from the buffer of c
						it := c.Buf[c.Rnext].Item
						partner := t.Events[1].Partner
						if partner == it.Thread { //is it the right partner thread?
							// is it the right send from the partner thread?
							// loop because it could be a select statement
							for _, op2 := range it.Ops {
								if op2.PC == t.Events[1].Ops[0].PPC {
									return []util.SyncPair{util.SyncPair{T1: e.Thread, T2: op.Ch, AsyncRcv: true, IsSelect: len(e.Ops) > 1}}
								}
							}
						}
					}
				}
				continue
			}

			// sync channel case
			if op.Kind&util.RCV > 0 { //rcver determines the next sender
				if len(t.Events) == 1 {
					continue
				}
				partner := t.Events[1].Partner
				ppc := t.Events[1].Ops[0].PPC

				for _, p := range m.Threads {
					if p.ID == e.Thread || len(p.Events) == 0 || partner != p.ID {
						continue
					}
					pe := p.Peek()
					for j, pop := range pe.Ops {
						if pop.Ch == op.Ch && pop.Kind == util.PREPARE|util.SEND && ppc == pop.PC {
							state := m.ChanVC[op.Ch]
							if state == nil {
								state = util.NewChanState()
							}
							state.SndCnt++
							m.ChanVC[op.Ch] = state
							return []util.SyncPair{util.SyncPair{T1: e.Thread, T2: p.ID, Idx: i, T2Idx: j, Sync: true, IsSelect: len(e.Ops) > 1}}
						}
					}
				}
			}

			if op.Kind&util.CLS > 0 {
				//check if its time for the close or not
				state := m.ChanVC[op.Ch]
				if state == nil {
					state = util.NewChanState()
				}
				if state.SndCnt == op.BufField {
					state.SndCnt++
					m.ChanVC[op.Ch] = state
					return []util.SyncPair{util.SyncPair{T1: e.Thread, T2: op.Ch, DoClose: true, Idx: i}}
				}

			}
		}
	}

	return nil
}

func (m *Machine) AllowNextAccess() {
	m.AccessCounter++
}
