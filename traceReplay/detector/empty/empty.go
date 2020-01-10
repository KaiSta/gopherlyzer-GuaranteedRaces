package empty

import (
	"fmt"

	"../../util"
	"../analysis"
)

type EventCollector struct{}

func Init() {
	vars = make(map[uint32]struct{})
	threads = make(map[uint32]struct{})
	locks = make(map[uint32]struct{})

	algos.RegisterDetector("empty", &EventCollector{})
}

var reads uint
var writes uint
var syncs uint
var vars map[uint32]struct{}
var threads map[uint32]struct{}
var locks map[uint32]struct{}
var all uint

func (l *EventCollector) Put(p *util.SyncPair) {
	all++
	if p.Lock {
		syncs++
		locks[p.T2] = struct{}{}
		threads[p.T1] = struct{}{}
	} else if p.Write {
		writes++
		vars[p.T2] = struct{}{}
		threads[p.T1] = struct{}{}
	} else if p.Read {
		reads++
		vars[p.T2] = struct{}{}
		threads[p.T1] = struct{}{}
	} else if p.PostProcess {
		fmt.Printf("\nReads:%v\nWrites:%v\nSyncs:%v\nVars:%v\nThreads:%v\nLocks:%v\nAll:%v\n",
			reads, writes, syncs, len(vars), len(threads), len(locks), all)
	}

}
