package tsanwrd

type thread struct {
	vc vcepoch
	//ls        map[uint32]struct{}
	ls        []uint32
	posLock   map[uint32]int
	csHistory map[uint32][]vcPair
	id        uint32
	deps      []intpair
}

func newT(id uint32) *thread {
	t := &thread{id: id, vc: newvc2().set(id, 1), ls: make([]uint32, 0), /*ls: make(map[uint32]struct{})*/
		posLock: make(map[uint32]int), csHistory: make(map[uint32][]vcPair)}

	if len(lockClues) > 0 {
		for k, l := range locks {
			list, ok := t.csHistory[k]
			if !ok {
				list = make([]vcPair, 0)
			}

			for i := 0; i < l.count; i++ {
				list = append(list, l.history[i])
			}

			t.csHistory[k] = list
		}
	} else {
		for k, l := range locks {
			list, ok := t.csHistory[k]
			if !ok {
				list = make([]vcPair, 0)
			}
			list = append(list, l.history...)

			t.csHistory[k] = list
		}
	}
	return t
}

type lock struct {
	rel        vcepoch
	rels       []vcepoch
	hb         vcepoch
	history    []vcPair
	acq        epoch
	nextAcq    epoch
	count      int
	strongSync bool
	fullSync   bool
}

func (l *lock) reset() {
	l.rel = newvc2()
	l.hb = newvc2()
	l.count = 0
}

func newL() *lock {
	return &lock{rel: newvc2(), hb: newvc2(), history: make([]vcPair, 0)}
}

type variable struct {
	races       []datarace
	history     []variableHistory
	frontier    []*dot
	graph       *fsGraph
	lastWrite   vcepoch
	lwLocks     map[uint32]struct{}
	lwOpenLocks []intpair
	lwDot       *dot
	current     int
}

func newVar() *variable {
	return &variable{lastWrite: newvc2(), lwDot: nil, frontier: make([]*dot, 0),
		current: 0, graph: newGraph(), races: make([]datarace, 0),
		history: make([]variableHistory, 0), lwLocks: make(map[uint32]struct{})}
}
