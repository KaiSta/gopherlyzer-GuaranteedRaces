package util

import "fmt"

type OpKind int

func (k OpKind) String() string {
	sym := "!"
	stat := "P"

	if k&RCV > 0 {
		if k&SEND > 0 {
			sym = "?!"
		} else {
			sym = "?"
		}

	} else if k&CLS > 0 {
		sym = "#"
	}

	if k&COMMIT > 0 {
		stat = "C"
	}

	if k&WAIT > 0 {
		sym = "W"
		stat = "C"
	} else if k&SIG > 0 {
		sym = "S"
		stat = "C"
	}

	if k&WRITE > 0 {
		sym = "M"
		stat = "C"
	} else if k&READ > 0 {
		sym = "R"
		stat = "C"
	}
	return fmt.Sprintf("%v,%v", sym, stat)
}

type Operation struct {
	PC        uint64
	PPC       uint64
	Ch        uint32
	BufSize   uint32
	BufField  uint32
	SourceRef uint32
	Line      uint32
	Kind      OpKind
	Mutex     int
}

func (o Operation) String() string {
	return fmt.Sprintf("%v(%v|%v),%v,%v:%v,%v,%v", o.Ch, o.BufSize, o.BufField, o.Kind, o.SourceRef, o.Line, o.PC, o.PPC)
}

func (o Operation) Clone() Operation {
	return Operation{Ch: o.Ch, Kind: o.Kind, BufSize: o.BufSize, BufField: o.BufField, SourceRef: o.SourceRef, Line: o.Line, Mutex: o.Mutex, PC: o.PC, PPC: o.PPC}
}

type Item struct {
	Ops      []Operation
	Thread   uint32
	Partner  uint32
	LocalIdx int
}

func (o Item) Clone() *Item {
	var ops []Operation
	for _, x := range o.Ops {
		ops = append(ops, x)
	}
	//vc := NewVC()
	// for k, v := range o.VC {
	// 	vc[k] = v
	// }
	// next := make([]*Item, len(o.Next))
	// for i := range o.Next {
	// 	next[i] = o.Next[i]
	// }
	// return &Item{o.Thread, ops, o.Partner, next, o.Prev, o.InternalNext, o.NextSndRcv, o.LocalIdx}
	return &Item{Thread: o.Thread, Ops: ops, Partner: o.Partner, LocalIdx: o.LocalIdx}
}

func (o Item) String() string {
	var ops string
	for i, p := range o.Ops {
		ops += fmt.Sprintf("(%v)", p)

		if i+1 < len(o.Ops) {
			ops += ","
		}
	}

	if o.Partner != 0 {
		return fmt.Sprintf("%v,[%v],%v", o.Thread, ops, o.Partner)
	}
	return fmt.Sprintf("%v,[%v]", o.Thread, ops)
}
func (o Item) ShortString() string {
	var ops string
	for i, p := range o.Ops {
		ops += fmt.Sprintf("(%v)", p)

		if i+1 < len(o.Ops) {
			ops += ","
		}
	}

	if o.Partner != 0 {
		return fmt.Sprintf("%v,[%v],%v", o.Thread, ops, o.Partner)
	}
	return fmt.Sprintf("%v,[%v]", o.Thread, ops)
}

type Thread struct {
	Events []*Item
	ID     uint32
}

func (t Thread) String() string {
	return fmt.Sprintf("(%v, %v)", t.ID, t.Events)
}
func (t Thread) ShortString() string {
	return fmt.Sprintf("(%v, %v)", t.ID, t.Events[0])
}

func (t Thread) Clone() Thread {
	var items []*Item
	for i := range t.Events {
		items = append(items, t.Events[i])
	}
	// vc := NewVC()
	// for k, v := range t.VC {
	// 	vc[k] = v
	// }
	// var mset map[uint64]struct{}
	// for k, v := range t.MutexSet {
	// 	mset[k] = v
	// }
	// rvc := NewVC()
	// for k, v := range t.RVC {
	// 	rvc[k] = v
	// }
	return Thread{ID: t.ID, Events: items}
	//	return Thread{t.ID, t.isBlocked, t.done, items, t.systemState}
}

func (t Thread) Peek() *Item {
	return t.Events[0]
}
func (t *Thread) Pop() {
	if len(t.Events) > 1 {
		t.Events = t.Events[1:]
	} else {
		t.Events = []*Item{}
	}
}

type VectorClock map[uint64]int

func (vc VectorClock) String() string {
	s := "["
	for k, v := range vc {
		s += fmt.Sprintf("(T%v@%v)", k, v)
	}
	s += "]"
	return s
}
func (vc VectorClock) Sync(pvc VectorClock) {
	for k, v := range vc {
		pv := pvc[k]
		tmp := Max(v, pv)
		//vc[k] = Max(v, pv)
		vc[k] = tmp
		pvc[k] = tmp
	}
	for k, v := range pvc {
		pv := vc[k]
		tmp := Max(v, pv)
		vc[k] = tmp
		pvc[k] = tmp
	}
}

func (vc VectorClock) Equals(pvc VectorClock) bool {
	if len(vc) != len(pvc) {
		return false
	}
	for k, v := range vc {
		pv := pvc[k]
		if v != pv {
			return false
		}
	}
	return true
}

func (vc VectorClock) Remove(pvc VectorClock) VectorClock {
	nClock := NewVC()
	for k, v := range vc {
		w := pvc[k]
		if v > w {
			nClock[k] = v
		}
	}
	for k, w := range pvc {
		_, ok := vc[k]
		if !ok {
			vc[k] = w
		}
	}

	return nClock
}

func (vc VectorClock) AddEpoch(ep Epoch) {
	vc[ep.X] = ep.T
}

func (vc VectorClock) Add(k uint64, val int) {
	v := vc[k]
	v += val
	vc[k] = v
}
func (vc VectorClock) Set(k uint64, val int) {
	vc[k] = val
}
func (vc VectorClock) Less(pvc VectorClock) bool {
	if len(vc) == 0 { //??? not sure if that is ok
		return true
	}
	f := false
	for k := range vc {
		if vc[k] > pvc[k] {
			return false
		}
		if vc[k] < pvc[k] {
			f = true
		}
	}
	for k := range pvc {
		if vc[k] > pvc[k] {
			return false
		}
		if vc[k] < pvc[k] {
			f = true
		}
	}
	return f
}

func (vc VectorClock) HasConflict(pvc VectorClock) bool {
	for k := range vc {
		if pvc[k] < vc[k] {
			return false
		}
	}
	return true
}

func (vc VectorClock) FindConflict(pvc VectorClock) uint64 {
	for k := range vc {
		if vc[k] > pvc[k] {
			return k
		}
	}
	for k := range pvc {
		if vc[k] > pvc[k] {
			return k
		}
	}
	return 0
}

func (vc VectorClock) ConcurrentTo(pvc VectorClock) bool {
	if vc.Less(pvc) || pvc.Less(vc) {
		return false
	}
	return true
}

func (vc VectorClock) Less_Epoch(pepoch Epoch) bool {
	return vc[pepoch.X] < pepoch.T
}

func (vc VectorClock) Clone() VectorClock {
	nvc := NewVC()
	for k, v := range vc {
		nvc[k] = v
	}
	return nvc
}

func NewVC() VectorClock {
	return make(VectorClock)
}

type Result struct {
	Alts []Alternative2
	POs  []Alternative2
}
type Alternative struct {
	Op     string   `json:"op"`
	Used   []string `json:"used"`
	Unused []string `json:"unused"`
}
type Alternative2 struct {
	Op     *Item
	Used   []*Item
	Unused []*Item
}

type VarState1 struct {
	Rvc     VectorClock
	Wvc     VectorClock
	PrevREv *Item
	PrevWEv *Item
}

type Epoch struct {
	X uint64
	T int
}

func NewEpoch(x uint64, t int) Epoch {
	return Epoch{x, t}
}
func (e *Epoch) Set(x uint64, t int) {
	e.X = x
	e.T = t
}
func (e Epoch) Less_Epoch(vc VectorClock) bool {
	return e.T < vc[e.X]
}

type VarState2 struct {
	Rvc     VectorClock
	Wepoch  Epoch
	Repoch  Epoch
	PrevREv *Item
	PrevWEv *Item
}

type VState int

func (s VState) String() string {
	switch s {
	case EXCLUSIVE:
		return "EXCLUSIVE"
	case READSHARED:
		return "READSHARED"
	case SHARED:
		return "SHARED"
	}
	return ""
}

//VarState3 for Epoch + Eraser solution
type VarState3 struct {
	Rvc        VectorClock
	Wvc        VectorClock
	Wepoch     Epoch
	Repoch     Epoch
	State      VState
	LastAccess uint64
	LastOp     *Operation
	LastEv     *Item
	LastSP     *SyncPair
	MutexSet   map[uint64]struct{}
	PrevREv    *Item
	PrevWEv    *Item
	TSet       VectorClock
}

type ChanState struct {
	Rvc      VectorClock
	Wvc      VectorClock
	WContext map[uint64]string
	RContext map[uint64]string
	//For RWLocks
	CountR int
	SndCnt uint32
}

func NewChanState() *ChanState {
	//return &ChanState{Rvc: NewVC(), Wvc: NewVC(), WContext: make([]string, 0), RContext: make([]string, 0)}
	return &ChanState{Rvc: NewVC(), Wvc: NewVC(), WContext: make(map[uint64]string), RContext: make(map[uint64]string)}
}

type BufField struct {
	*Item
	VC VectorClock
}

type AsyncChan struct {
	Buf        []BufField
	BufSize    uint32
	Next       uint32
	Count      uint32
	Rnext      uint32
	IsLock     bool
	Rcounter   uint32
	SndCounter uint32
}

type SyncPair struct {
	T1           uint32
	T2           uint32
	AsyncSend    bool
	AsyncRcv     bool
	Closed       bool
	DoClose      bool
	GoStart      bool
	DataAccess   bool
	Write        bool
	Read         bool
	AtomicWrite  bool
	AtomicRead   bool
	Lock         bool
	Unlock       bool
	Sync         bool
	IsSelect     bool
	Idx          int
	T2Idx        int
	IsGoStart    bool
	IsFork       bool
	IsWait       bool
	IsPreBranch  bool
	IsPostBranch bool
	IsNT         bool
	IsNTWT       bool
	PostProcess  bool
	Counter      uint64
	Ev           *Item
}

func (s SyncPair) String() string {
	if s.AsyncSend {
		return fmt.Sprintf("Thread %v send async to chan %v", s.T1, s.T2)
	} else if s.AsyncRcv {
		return fmt.Sprintf("Thread %v received async from chan %v", s.T1, s.T2)
	} else if s.Closed {
		return fmt.Sprintf("Thread %v operates on closed chan %v", s.T1, s.T2)
	} else if s.DoClose {
		return fmt.Sprintf("Thread %v closed channel %v", s.T1, s.T2)
	} else if s.DataAccess {
		return fmt.Sprintf("Thread %v accessed var %v (%v)", s.T1, s.T2, s.Idx)
	} else if s.Sync {
		return fmt.Sprintf("Thread %v synced with Thread %v", s.T2, s.T1)
	} else if s.IsFork {
		return fmt.Sprintf("Thread %v forked with id %v", s.T1, s.T2)
	} else if s.IsWait {
		return fmt.Sprintf("Thread %v waits with id %v", s.T1, s.T2)
	} else if s.Write {
		return fmt.Sprintf("Thread %v writes var %v (%v)", s.T1, s.T2, s.Idx)
	} else if s.Read {
		return fmt.Sprintf("Thread %v reads var %v (%v)", s.T1, s.T2, s.Idx)
	} else if s.Lock {
		return fmt.Sprintf("Thread %v locks mutex %v (%v)", s.T1, s.T2, s.Idx)
	} else if s.Unlock {
		return fmt.Sprintf("Thread %v unlocks mutex %v (%v)", s.T1, s.T2, s.Idx)
	}
	return "UNKNOWN"
}
