package race

import (
	"fmt"
	"time"

	traceParser "../parser"
	"../util"
	algos "./analysis"
	"./empty"
	"./eraser"
	fastTrack "./fasttrack"
	"./report"
	svars "./sharedVars"
	"./shb"
	"./shbee"
	"./shbp"
	speedygov2 "./speedyGo/v2"
	sshb "./strictSHB"
	"./threadSanitizer"
	"./traceReplay"
)

func init() {
	fastTrack.Init()
	//	goTrack.Init()
	eraser.Init()
	threadSanitizer.Init()
	//	raceTrack.Init()
	shb.Init()
	//	speedygo.Init()
	empty.Init()
	svars.Init()
	speedygov2.Init()
	sshb.Init()
	shbee.Init()
	shbp.Init()
	// ucp.Init()
	// racequal.Init()
	// wcp.Init()
	// fsshbee.Init()
	// tsanee.Init()
	// wcpp.Init()
	// ptsanee.Init()
	// ultratsanee.Init()
}

var debugReplay = false

func startThreads(m *traceReplay.Machine) {
	for {
		threads := m.GetThreadStarts()
		if len(threads) == 0 {
			return
		}
		for k, v := range threads {
			s := &util.SyncPair{T1: k, T2: v, IsGoStart: true}
			for _, l := range traceReplay.EvListener {
				l.Put(s)
			}
		}
	}
}

func startThreads2(m *traceReplay.Machine) {
	sigs, waits := m.HandleSigNWait()

	for i := range sigs {
		for _, l := range traceReplay.EvListener {
			l.Put(&sigs[i])
		}
	}
	for i := range waits {
		for _, l := range traceReplay.EvListener {
			l.Put(&waits[i])
		}
	}
}

func replay(m *traceReplay.Machine, jsonFlag, plain, bench bool) {
	complete := 0
	for _, t := range m.Threads {
		complete += len(t.Events)
	}
	//var lastProg float64
	steps := 0

	stepper := traceReplay.Stepper{}

	for {
		//startThreads(m)
		//startThreads2(m)

		if debugReplay {
			fmt.Println("---------")
			for _, t := range m.Threads {
				if len(t.Events) > 0 {
					fmt.Println(t.Peek())
				}
			}
			fmt.Println("----------")
		}

		steps++
		if steps%1000 == 0 {
			fmt.Printf("\r%v/%v", steps, complete)
		}

		// if currProg-lastProg >= 5.0 {
		// 	fmt.Println(currProg)
		// 	lastProg = currProg
		// } else if steps%5000 == 0 {
		// 	fmt.Println((float64(complete-current) / float64(complete)) * 100)
		// }

		pairs := m.GetNextActionv2()

		if debugReplay {
			for _, p := range pairs {
				fmt.Println(">>", p)
			}
		}

		if len(pairs) == 0 {
			break
		}

		for _, l := range traceReplay.EvListener {
			l.Put(&pairs[0])
		}
		stepper.Put(m, &pairs[0])

	}
	for _, l := range traceReplay.EvListener {
		l.Put(&util.SyncPair{PostProcess: true})
	}

	//if debugReplay {
	fmt.Println("---------")

	for _, t := range m.Threads {
		if len(t.Events) > 0 {
			fmt.Println(t.Peek())
		}
	}
	fmt.Println("AccessCounter:", m.AccessCounter)
	fmt.Println("LockCounter:", m.LockCounter)

	fmt.Println("----------")
	//	}

	leftovers := 0
	for _, t := range m.Threads {
		leftovers += len(t.Events)
	}
	fmt.Println("Leftovers:", leftovers)
}

func filter1(items []util.Item) []util.Item {
	algos.FilterUnshared(items)
	nItems := make([]util.Item, 0, len(items)/2)

	for _, it := range items {
		if it.Ops[0].Kind&util.WRITE > 0 || it.Ops[0].Kind&util.READ > 0 || it.Ops[0].Kind&util.ATOMICWRITE > 0 || it.Ops[0].Kind&util.ATOMICREAD > 0 {
			if v, ok := algos.Variables[it.Ops[0].Ch]; ok && v.Shared {
				nItems = append(nItems, it)
			}
		} else {
			nItems = append(nItems, it)
		}
	}

	return nItems
}

func filter2(items []util.Item) []util.Item {
	nItems := make([]util.Item, 0, 1000)

	vmap := make(map[uint32]uint32)

	for _, it := range items {
		if it.Ops[0].Kind&util.WRITE > 0 || it.Ops[0].Kind&util.READ > 0 || it.Ops[0].Kind&util.ATOMICWRITE > 0 || it.Ops[0].Kind&util.ATOMICREAD > 0 {
			if t, ok := vmap[it.Ops[0].Ch]; ok {
				if t != it.Thread {
					nItems = append(nItems, it)
					vmap[it.Ops[0].Ch] = it.Thread
				}
			} else {
				nItems = append(nItems, it)
				vmap[it.Ops[0].Ch] = it.Thread
			}
		} else {
			nItems = append(nItems, it)
		}
	}
	return nItems
}

func RunAPISingleInc(detector string, withPostProcess bool, trace string, n uint64) {
	traceReplay.EvListener = []traceReplay.EventListener{algos.GetDetector(detector)}

	items := make(chan *util.Item, 100000)
	go traceParser.ParseJTracevInc(trace, n, items)

	steps := 0
	//complete := len(items)
	stepBarrier := 5000

	SignalList2 := make(map[uint32]*util.Item)

	var event util.SyncPair
	t1 := time.Now()
	for it := range items {
		if debugReplay {
			fmt.Println(it)
		}

		op := it.Ops[0]
		if op.Kind&util.WRITE > 0 || op.Kind&util.READ > 0 {
			event = util.SyncPair{T1: it.Thread, T2: op.Ch, DataAccess: true, Write: op.Kind&util.WRITE > 0,
				Read: op.Kind&util.READ > 0, Ev: it}
		} else if op.Kind&util.ATOMICWRITE > 0 || op.Kind&util.ATOMICREAD > 0 {
			event = util.SyncPair{T1: it.Thread, T2: op.Ch, DataAccess: true, AtomicWrite: op.Kind&util.WRITE > 0,
				AtomicRead: op.Kind&util.READ > 0, Ev: it}
		} else if op.Kind&util.SIG > 0 {
			SignalList2[op.Ch] = it
			event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsFork: true, Ev: it}
		} else if op.Kind&util.WAIT > 0 {
			_, ok := SignalList2[op.Ch]
			if ok {
				event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsWait: true, Ev: it}
			}
			// } else if op.Kind&util.PREBRANCH > 0 {
			// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsPreBranch: true, Ev: it}
			// } else if op.Kind&util.POSTBRANCH > 0 {
			// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsPostBranch: true, Ev: it}
			// } else if op.Kind&util.NT > 0 {
			// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsNT: true, Ev: it}
			// } else if op.Kind&util.NTWT > 0 {
			// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsNTWT: true, Ev: it}
		} else if op.BufSize > 0 {
			if op.Mutex == util.LOCK {
				event = util.SyncPair{T1: it.Thread, T2: op.Ch, Lock: true, AsyncSend: true, Ev: it}
			} else if op.Mutex == util.UNLOCK {
				event = util.SyncPair{T1: it.Thread, T2: op.Ch, Unlock: true, AsyncRcv: true, Ev: it}
			}
		}

		steps++
		if steps%stepBarrier == 0 {
			fmt.Printf("\r%v/%v", steps, "?")
		}

		if debugReplay {
			fmt.Println(event)
		}
		for _, l := range traceReplay.EvListener {
			l.Put(&event)
		}
	}

	t2 := time.Now()
	fmt.Println("Phase1 Time:", t2.Sub(t1))

	if withPostProcess {
		t1 := time.Now()
		for _, l := range traceReplay.EvListener {
			l.Put(&util.SyncPair{PostProcess: true})
		}
		t2 := time.Now()
		fmt.Println("PostProcessing Time:", t2.Sub(t1))
	}
	report.ReportNumbers()
}

func RunAPISingleIncDouble(detector string, withPostProcess bool, trace string, n uint64) {
	traceReplay.EvListener = []traceReplay.EventListener{algos.GetDetector(detector)}

	for i := 0; i < 2; i++ {
		items := make(chan *util.Item, 100000)
		go traceParser.ParseJTracevInc(trace, n, items)

		steps := 0
		//complete := len(items)
		stepBarrier := 5000

		SignalList2 := make(map[uint32]*util.Item)

		var event util.SyncPair
		t1 := time.Now()
		for it := range items {
			if debugReplay {
				fmt.Println(it)
			}

			op := it.Ops[0]
			if op.Kind&util.WRITE > 0 || op.Kind&util.READ > 0 {
				event = util.SyncPair{T1: it.Thread, T2: op.Ch, DataAccess: true, Write: op.Kind&util.WRITE > 0,
					Read: op.Kind&util.READ > 0, Ev: it}
			} else if op.Kind&util.ATOMICWRITE > 0 || op.Kind&util.ATOMICREAD > 0 {
				event = util.SyncPair{T1: it.Thread, T2: op.Ch, DataAccess: true, AtomicWrite: op.Kind&util.WRITE > 0,
					AtomicRead: op.Kind&util.READ > 0, Ev: it}
			} else if op.Kind&util.SIG > 0 {
				SignalList2[op.Ch] = it
				event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsFork: true, Ev: it}
			} else if op.Kind&util.WAIT > 0 {
				_, ok := SignalList2[op.Ch]
				if ok {
					event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsWait: true, Ev: it}
				}
				// } else if op.Kind&util.PREBRANCH > 0 {
				// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsPreBranch: true, Ev: it}
				// } else if op.Kind&util.POSTBRANCH > 0 {
				// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsPostBranch: true, Ev: it}
				// } else if op.Kind&util.NT > 0 {
				// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsNT: true, Ev: it}
				// } else if op.Kind&util.NTWT > 0 {
				// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsNTWT: true, Ev: it}
			} else if op.BufSize > 0 {
				if op.Mutex == util.LOCK {
					event = util.SyncPair{T1: it.Thread, T2: op.Ch, Lock: true, AsyncSend: true, Ev: it}
				} else if op.Mutex == util.UNLOCK {
					event = util.SyncPair{T1: it.Thread, T2: op.Ch, Unlock: true, AsyncRcv: true, Ev: it}
				}
			}

			steps++
			if steps%stepBarrier == 0 {
				fmt.Printf("\r%v/%v", steps, "?")
			}

			if debugReplay {
				fmt.Println(event)
			}
			for _, l := range traceReplay.EvListener {
				l.Put(&event)
			}
		}

		t2 := time.Now()
		fmt.Println("Phase1 Time:", t2.Sub(t1))

		if withPostProcess {
			t1 := time.Now()
			for _, l := range traceReplay.EvListener {
				l.Put(&util.SyncPair{PostProcess: true})
			}
			t2 := time.Now()
			fmt.Println("PostProcessing Time:", t2.Sub(t1))
		}
		report.ReportNumbers()
	}
}

func RunAPISingle(items []util.Item, detector string, filter uint64, withPostProcess bool) {
	if filter > 0 {
		fmt.Println("Item count before:", len(items))
		if filter == 1 || filter == 2 {
			items = filter1(items)
		}
		if filter == 2 {
			items = filter2(items)
		}
		fmt.Println("Item count after:", len(items))
	}

	steps := 0
	complete := len(items)
	stepBarrier := (complete / 20) + 1

	traceReplay.EvListener = []traceReplay.EventListener{algos.GetDetector(detector)}

	SignalList2 := make(map[uint32]*util.Item)

	var event util.SyncPair
	t1 := time.Now()
	for i, it := range items {
		if debugReplay {
			fmt.Println(it)
		}

		op := it.Ops[0]
		if op.Kind&util.WRITE > 0 || op.Kind&util.READ > 0 {
			event = util.SyncPair{T1: it.Thread, T2: op.Ch, DataAccess: true, Write: op.Kind&util.WRITE > 0,
				Read: op.Kind&util.READ > 0, Ev: &items[i]}
		} else if op.Kind&util.ATOMICWRITE > 0 || op.Kind&util.ATOMICREAD > 0 {
			event = util.SyncPair{T1: it.Thread, T2: op.Ch, DataAccess: true, AtomicWrite: op.Kind&util.WRITE > 0,
				AtomicRead: op.Kind&util.READ > 0, Ev: &items[i]}
		} else if op.Kind&util.SIG > 0 {
			SignalList2[op.Ch] = &items[i]
			event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsFork: true, Ev: &items[i]}
		} else if op.Kind&util.WAIT > 0 {
			_, ok := SignalList2[op.Ch]
			if ok {
				event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsWait: true, Ev: &items[i]}
			}
			// } else if op.Kind&util.PREBRANCH > 0 {
			// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsPreBranch: true, Ev: &items[i]}
			// } else if op.Kind&util.POSTBRANCH > 0 {
			// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsPostBranch: true, Ev: &items[i]}
			// } else if op.Kind&util.NT > 0 {
			// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsNT: true, Ev: &items[i]}
			// } else if op.Kind&util.NTWT > 0 {
			// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsNTWT: true, Ev: &items[i]}
		} else if op.BufSize > 0 {
			if op.Mutex == util.LOCK {
				event = util.SyncPair{T1: it.Thread, T2: op.Ch, Lock: true, AsyncSend: true, Ev: &items[i]}
			} else if op.Mutex == util.UNLOCK {
				event = util.SyncPair{T1: it.Thread, T2: op.Ch, Unlock: true, AsyncRcv: true, Ev: &items[i]}
			}
		}

		steps++
		if steps%stepBarrier == 0 {
			fmt.Printf("\r%v/%v", steps, complete)
		}

		if debugReplay {
			fmt.Println(event)
		}
		for _, l := range traceReplay.EvListener {
			l.Put(&event)
		}
	}
	t2 := time.Now()
	fmt.Println("Phase1 Time:", t2.Sub(t1))

	if withPostProcess {
		t1 := time.Now()
		for _, l := range traceReplay.EvListener {
			l.Put(&util.SyncPair{PostProcess: true})
		}
		t2 := time.Now()
		fmt.Println("PostProcessing Time:", t2.Sub(t1))
	}
	report.ReportNumbers()
}

func RunAPISingleDouble(items []util.Item, detector string, filter uint64, withPostProcess bool) {
	if filter > 0 {
		fmt.Println("Item count before:", len(items))
		if filter == 1 || filter == 2 {
			items = filter1(items)
		}
		if filter == 2 {
			items = filter2(items)
		}
		fmt.Println("Item count after:", len(items))
	}

	traceReplay.EvListener = []traceReplay.EventListener{algos.GetDetector(detector)}

	for i := 0; i < 2; i++ {
		steps := 0
		complete := len(items)
		stepBarrier := (complete / 20) + 1

		SignalList2 := make(map[uint32]*util.Item)

		var event util.SyncPair
		t1 := time.Now()
		for i, it := range items {
			if debugReplay {
				fmt.Println(it)
			}

			op := it.Ops[0]
			if op.Kind&util.WRITE > 0 || op.Kind&util.READ > 0 {
				event = util.SyncPair{T1: it.Thread, T2: op.Ch, DataAccess: true, Write: op.Kind&util.WRITE > 0,
					Read: op.Kind&util.READ > 0, Ev: &items[i]}
			} else if op.Kind&util.ATOMICWRITE > 0 || op.Kind&util.ATOMICREAD > 0 {
				event = util.SyncPair{T1: it.Thread, T2: op.Ch, DataAccess: true, AtomicWrite: op.Kind&util.WRITE > 0,
					AtomicRead: op.Kind&util.READ > 0, Ev: &items[i]}
			} else if op.Kind&util.SIG > 0 {
				SignalList2[op.Ch] = &items[i]
				event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsFork: true, Ev: &items[i]}
			} else if op.Kind&util.WAIT > 0 {
				_, ok := SignalList2[op.Ch]
				if ok {
					event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsWait: true, Ev: &items[i]}
				}
				// } else if op.Kind&util.PREBRANCH > 0 {
				// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsPreBranch: true, Ev: &items[i]}
				// } else if op.Kind&util.POSTBRANCH > 0 {
				// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsPostBranch: true, Ev: &items[i]}
				// } else if op.Kind&util.NT > 0 {
				// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsNT: true, Ev: &items[i]}
				// } else if op.Kind&util.NTWT > 0 {
				// 	event = util.SyncPair{T1: it.Thread, T2: op.Ch, IsNTWT: true, Ev: &items[i]}
			} else if op.BufSize > 0 {
				if op.Mutex == util.LOCK {
					event = util.SyncPair{T1: it.Thread, T2: op.Ch, Lock: true, AsyncSend: true, Ev: &items[i]}
				} else if op.Mutex == util.UNLOCK {
					event = util.SyncPair{T1: it.Thread, T2: op.Ch, Unlock: true, AsyncRcv: true, Ev: &items[i]}
				}
			}

			steps++
			if steps%stepBarrier == 0 {
				fmt.Printf("\r%v/%v", steps, complete)
			}

			if debugReplay {
				fmt.Println(event)
			}
			for _, l := range traceReplay.EvListener {
				l.Put(&event)
			}
		}
		t2 := time.Now()
		fmt.Println("Phase1 Time:", t2.Sub(t1))

		if withPostProcess {
			t1 := time.Now()
			for _, l := range traceReplay.EvListener {
				l.Put(&util.SyncPair{PostProcess: true})
			}
			t2 := time.Now()
			fmt.Println("PostProcessing Time:", t2.Sub(t1))
		}
		report.ReportNumbers()
	}
}

func LSDetectors() {
	algos.PrintRegisteredDetectors()
}

func RunAPIV2(items []util.Item, detector string, filter uint64) {
	if filter > 0 {
		fmt.Println("Item count before:", len(items))
		if filter == 1 || filter == 2 {
			items = filter1(items)
		}
		if filter == 2 {
			items = filter2(items)
		}
		fmt.Println("Item count after:", len(items))
	}

	fmt.Println("Creating threads...")
	threads := createThreads(items)
	fmt.Println("Created threads!")

	fmt.Println("Creating async chans...")
	asyncChans := getAsyncChans2(items)
	fmt.Println("Created async chans")
	closedChans := make(map[uint32]struct{})
	closedChans[0] = struct{}{}

	m := &traceReplay.Machine{threads, nil, closedChans, make(map[uint32]util.VectorClock),
		false, nil, nil,
		make(map[uint32]*util.VarState3),
		asyncChans,
		make(map[uint32]*util.ChanState),
		make([]traceReplay.SelectStore, 0), 1, 1, 1, make([]*util.Item, 0),
		make(map[uint32]*util.Item)}

	traceReplay.EvListener = []traceReplay.EventListener{algos.GetDetector(detector)}
	fmt.Println("Starting replay...")
	replay(m, false, false, false)
	fmt.Println("Replay done!")

	fmt.Println("Results with filter level:", filter)
	report.ReportNumbers()
}
