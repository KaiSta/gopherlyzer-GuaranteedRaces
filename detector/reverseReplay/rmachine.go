package reverseReplay

import (
	"fmt"

	"../../util"
	"../traceReplay"
)

type DataRace struct {
	T1    uint64
	T2    uint64
	T1Idx int
	T2Idx int
}

var uniqueDataRaces map[string]DataRace

func AddDataRace(race DataRace) {
	s := fmt.Sprintf("%v:%v,%v%v", race.T1, race.T1Idx, race.T2, race.T2Idx)

	_, ok := uniqueDataRaces[s]

	if !ok { //new data race
		uniqueDataRaces[s] = race
	}
}

type RMachine struct {
	Threads       map[uint64]util.Thread
	AsyncChans    map[uint64][]*util.Item
	ClosedChans   map[uint64]struct{}
	ClosedChansVC map[uint64]util.VectorClock
	Stopped       bool
	Vars1         map[uint64]*util.VarState1
	Vars2         map[uint64]*util.VarState2
	Vars3         map[uint64]*util.VarState3
	AsyncChans2   map[uint64]util.AsyncChan
	ChanVC        map[uint64]*util.ChanState
	//Selects       []SelectStore
	AccessCounter uint64
	FieldCounter  uint32
	LockCounter   uint32
	SignalList    []*util.Item
}

func (rm *RMachine) Configure(race DataRace) {
	t := rm.Threads[race.T1]
	//cut to race event
	t.Events = t.Events[:race.T1Idx+1]
	//reverse order
	rEvents := make([]*util.Item, 0, len(t.Events))
	s := len(rEvents) - 1
	for i := range t.Events {
		rEvents[s-i] = t.Events[i]
	}
	t.Events = rEvents
	rm.Threads[race.T1] = t

	{
		t := rm.Threads[race.T2]
		t.Events = t.Events[:race.T2Idx+1]
		//reverse order
		rEvents := make([]*util.Item, 0, len(t.Events))
		s = len(rEvents) - 1
		for i := range t.Events {
			rEvents[s-i] = t.Events[i]
		}
		t.Events = rEvents
		rm.Threads[race.T2] = t
	}

	//clean pending pre events from the top of the thread local traces
	for k, v := range rm.Threads {
		if v.Events[0].Ops[0].Kind&util.PREPARE > 0 {
			//remove first event
			v.Events = v.Events[1:]
			rm.Threads[k] = v
		}
	}
}

/*
  for rwd we need the highest counter of the subtrace that we observe. We need the same
  for async channels and sync channels (for close). reverse replay succeeds if it
  runs through completely with respect to all rwds?. If it fails then we have found
  a false positive?
*/

func (rm *RMachine) GetNextAction() []traceReplay.SyncPair {
	for _, t := range rm.Threads {
		if len(t.Events) == 0 {
			continue
		}
		e := t.Peek()

		for i, op := range e.Ops {
			if op.Kind&util.WRITE > 0 || op.Kind&util.READ > 0 {
				return []traceReplay.SyncPair{traceReplay.SyncPair{T1: t.ID, T2: op.Ch, DataAccess: true, Idx: i}}
			}

			if op.BufSize > 0 {
				c := rm.AsyncChans2[op.Ch]

				switch op.Kind {
				case util.POST | util.SEND:
					if op.Mutex == util.LOCK {
						return []traceReplay.SyncPair{traceReplay.SyncPair{T1: t.ID, T2: op.Ch, AsyncSend: true, IsSelect: len(e.Ops) > 1}}
					}

					if c.Count > 0 {
						it := c.Buf[c.Rnext].Item
						partner := it.Partner
						if partner == t.ID && op.PC == it.Ops[0].PPC {
							return []traceReplay.SyncPair{traceReplay.SyncPair{T1: t.ID, T2: op.Ch, AsyncSend: true, IsSelect: len(e.Ops) > 0}}
						}

					}
				case util.POST | util.RCV:
					if op.Mutex == util.UNLOCK && c.Count == 0 { // is a unlock and lockable
						return []traceReplay.SyncPair{traceReplay.SyncPair{T1: t.ID, T2: op.Ch, AsyncRcv: true, IsSelect: len(e.Ops) > 1}}
					}

					if c.Count < c.BufSize { //space in the buffer?
						return []traceReplay.SyncPair{traceReplay.SyncPair{T1: t.ID, T2: op.Ch, AsyncRcv: true, IsSelect: len(e.Ops) > 1}}
					}
				}
			}

			//sync chan
			if op.Kind&util.RCV > 0 { //rcver determines the next sender
				if len(t.Events) == 1 {
					continue
				}
				partner := t.Events[1].Partner
				ppc := t.Events[1].Ops[0].PPC

				for _, p := range rm.Threads {
					if p.ID == t.ID || len(p.Events) == 0 || partner != p.ID {
						continue
					}
					pe := p.Peek()
					for j, pop := range pe.Ops {
						if pop.Ch == op.Ch && pop.Kind == util.PREPARE|util.SEND && ppc == pop.PC {
							state := rm.ChanVC[op.Ch]
							state.SndCnt++
							rm.ChanVC[op.Ch] = state
							return []traceReplay.SyncPair{traceReplay.SyncPair{T1: t.ID, T2: p.ID, Idx: i, T2Idx: j, Sync: true, IsSelect: len(e.Ops) > 1}}
						}
					}
				}
			}

			if op.Kind&util.CLS > 0 {
				//check if its time for the close or not
				state := rm.ChanVC[op.Ch]
				if state.SndCnt == op.BufField {
					state.SndCnt++
					rm.ChanVC[op.Ch] = state
					return []traceReplay.SyncPair{traceReplay.SyncPair{T1: t.ID, T2: op.Ch, DoClose: true, Idx: i}}
				}

			}
		}
	}
	return nil
}
