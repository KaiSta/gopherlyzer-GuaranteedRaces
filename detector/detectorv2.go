package race

import (
	"fmt"
	"time"

	"../util"
	"./analysis"
	"./empty"
	"./eraser"
	"./fasttrack"
	"./report"
	"./sharedVars"
	"./shb"
	"./shbee"
	"./shbp"
	"./speedyGo/v2"
	"./strictSHB"
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

//OLD STUFF
// func RunFastTrack(tracePath string, json, plain, bench bool) {
// 	items := parser.ParseTracev2(tracePath)

// 	threads := createThreads(items)

// 	asyncChans := getAsyncChans2(items)
// 	closedChans := make(map[uint64]struct{})
// 	closedChans[0] = struct{}{}
// 	m := &traceReplay.Machine{threads, nil,
// 		closedChans, make(map[uint64]util.VectorClock),
// 		false,
// 		nil, nil,
// 		make(map[uint64]*util.VarState3),
// 		asyncChans,
// 		make(map[uint64]*util.ChanState),
// 		make([]traceReplay.SelectStore, 0), 1, 1, 0, make([]*util.Item, 0),
// 		make(map[uint64]*util.Item)}
// 	traceReplay.EvListener = []traceReplay.EventListener{
// 		&fastTrack.ListenerSelect{},
// 		&fastTrack.ListenerSync{},
// 		&fastTrack.ListenerAsyncSnd{},
// 		&fastTrack.ListenerAsyncRcv{},
// 		&fastTrack.ListenerChanClose{},
// 		&fastTrack.ListenerOpClosedChan{},
// 		&fastTrack.ListenerDataAccess2{},
// 		&fastTrack.ListenerGoFork{},
// 		&fastTrack.ListenerGoWait{},
// 	}
// 	replay(m, json, plain, bench)
// }

// func RunGoTrack(tracePath string, json, plain, bench bool) {
// 	items := parser.ParseTracev2(tracePath)

// 	threads := createThreads(items)

// 	asyncChans := getAsyncChans2(items)
// 	closedChans := make(map[uint64]struct{})
// 	closedChans[0] = struct{}{}
// 	m := &traceReplay.Machine{threads, nil,
// 		closedChans, make(map[uint64]util.VectorClock),
// 		false,
// 		nil, nil,
// 		make(map[uint64]*util.VarState3),
// 		asyncChans,
// 		make(map[uint64]*util.ChanState),
// 		make([]traceReplay.SelectStore, 0), 1, 1, 0, make([]*util.Item, 0),
// 		make(map[uint64]*util.Item)}
// 	traceReplay.EvListener = []traceReplay.EventListener{
// 		&goTrack.ListenerSelect{},
// 		&goTrack.ListenerSync{},
// 		&goTrack.ListenerAsyncSnd{},
// 		&goTrack.ListenerAsyncRcv{},
// 		&goTrack.ListenerChanClose{},
// 		&goTrack.ListenerOpClosedChan{},
// 		&goTrack.ListenerDataAccess2{},
// 		&goTrack.ListenerGoFork{},
// 		&goTrack.ListenerGoWait{},
// 	}
// 	replay(m, json, plain, bench)
// }

// func RunThreadSanitizer(tracePath string, json, plain, bench bool) {
// 	items := parser.ParseTracev2(tracePath)
// 	// for _, it := range items {
// 	// 	fmt.Println(it)
// 	// }
// 	threads := createThreads(items)

// 	// for _, t := range threads {
// 	// 	fmt.Println(t.ID)
// 	// 	for _, e := range t.Events {
// 	// 		fmt.Printf("\t%v\n", e)
// 	// 	}
// 	// }

// 	asyncChans := getAsyncChans2(items)
// 	closedChans := make(map[uint64]struct{})
// 	closedChans[0] = struct{}{}
// 	m := &traceReplay.Machine{threads, nil,
// 		closedChans, make(map[uint64]util.VectorClock),
// 		false,
// 		nil, nil,
// 		make(map[uint64]*util.VarState3),
// 		asyncChans,
// 		make(map[uint64]*util.ChanState),
// 		make([]traceReplay.SelectStore, 0), 1, 1, 0, make([]*util.Item, 0),
// 		make(map[uint64]*util.Item)}
// 	traceReplay.EvListener = []traceReplay.EventListener{
// 		&threadSanitizer.ListenerSelect{},
// 		&threadSanitizer.ListenerSync{},
// 		&threadSanitizer.ListenerAsyncSnd{},
// 		&threadSanitizer.ListenerAsyncRcv{},
// 		&threadSanitizer.ListenerChanClose{},
// 		&threadSanitizer.ListenerOpClosedChan{},
// 		&threadSanitizer.ListenerDataAccess{},
// 		&threadSanitizer.ListenerGoFork{},
// 		&threadSanitizer.ListenerGoWait{},
// 	}
// 	replay(m, json, plain, bench)
// }

// func RunEraser(tracePath string, json, plain, bench bool) {
// 	items := parser.ParseTracev2(tracePath)
// 	threads := createThreads(items)

// 	asyncChans := getAsyncChans2(items)
// 	closedChans := make(map[uint64]struct{})
// 	closedChans[0] = struct{}{}
// 	m := &traceReplay.Machine{threads, nil,
// 		closedChans, make(map[uint64]util.VectorClock),
// 		false,
// 		nil, nil,
// 		make(map[uint64]*util.VarState3),
// 		asyncChans,
// 		make(map[uint64]*util.ChanState),
// 		make([]traceReplay.SelectStore, 0), 1, 1, 0, make([]*util.Item, 0),
// 		make(map[uint64]*util.Item)}
// 	traceReplay.EvListener = []traceReplay.EventListener{
// 		&graphbuilder.ListenerGoFork{},
// 		&graphbuilder.ListenerGoWait{},
// 		&graphbuilder.ListenerSync{},
// 		&graphbuilder.ListenerAsyncSnd{},
// 		&graphbuilder.ListenerAsyncRcv{},
// 		&graphbuilder.ListenerDataAccess2{},
// 		&eraser.ListenerSelect{},
// 		&eraser.ListenerSync{},
// 		&eraser.ListenerAsyncSnd{},
// 		&eraser.ListenerAsyncRcv{},
// 		&eraser.ListenerChanClose{},
// 		&eraser.ListenerOpClosedChan{},
// 		&eraser.ListenerDataAccess{},
// 		&eraser.ListenerGoFork{},
// 		&eraser.ListenerGoWait{},
// 	}
// 	replay(m, json, plain, bench)

// 	// for _, it := range items {
// 	// 	if it.Ops[0].Kind&util.PREPARE > 0 {
// 	// 		continue
// 	// 	}
// 	// 	fmt.Println(it)
// 	// 	fmt.Println("\t", it.InternalNext)
// 	// 	for _, x := range it.Next {
// 	// 		fmt.Println("\t", x)
// 	// 	}
// 	// 	fmt.Println("prev", it.Prev)
// 	// 	fmt.Println("----")
// 	// }
// 	// for _, it := range graphbuilder.EmulatedThreads {
// 	// 	for _, buf := range it.Slots {
// 	// 		for _, ev := range buf.Events {
// 	// 			fmt.Println(ev)
// 	// 			fmt.Println("\t INTERNAL", ev.InternalNext)
// 	// 			for _, x := range ev.Next {
// 	// 				fmt.Println("\t", x)
// 	// 			}

// 	// 			fmt.Println("-----")
// 	// 		}
// 	// 	}
// 	// }

// 	for i, v := range report.UniqueDataRaces {
// 		fmt.Printf("%v\n\t%v\n\t%v\n\n", i, v.A, v.B)
// 		if CheckAlternatives2(v.A, v.B, graphbuilder.EmulatedThreads) {
// 			//		if Check(v.A, v.B) {
// 			fmt.Println("Real race!")
// 		} else {
// 			fmt.Println("Fake news!")
// 		}
// 	}
// }

// func RunRaceTrack(tracePath string, json, plain, bench bool) {
// 	items := parser.ParseTracev2(tracePath)
// 	threads := createThreads(items)

// 	asyncChans := getAsyncChans2(items)
// 	closedChans := make(map[uint64]struct{})
// 	closedChans[0] = struct{}{}
// 	m := &traceReplay.Machine{threads, nil,
// 		closedChans, make(map[uint64]util.VectorClock),
// 		false,
// 		nil, nil,
// 		make(map[uint64]*util.VarState3),
// 		asyncChans,
// 		make(map[uint64]*util.ChanState),
// 		make([]traceReplay.SelectStore, 0), 1, 1, 0, make([]*util.Item, 0),
// 		make(map[uint64]*util.Item)}
// 	traceReplay.EvListener = []traceReplay.EventListener{
// 		&raceTrack.ListenerSelect{},
// 		&raceTrack.ListenerSync{},
// 		&raceTrack.ListenerAsyncSnd{},
// 		&raceTrack.ListenerAsyncRcv{},
// 		&raceTrack.ListenerChanClose{},
// 		&raceTrack.ListenerOpClosedChan{},
// 		&raceTrack.ListenerDataAccess{},
// 		&raceTrack.ListenerGoStart{},
// 	}
// 	replay(m, json, plain, bench)
// }

// func RunTwinTrack(tracePath string, json, plain, bench bool) {
// 	items := parser.ParseTracev2(tracePath)
// 	threads := createThreads(items)

// 	asyncChans := getAsyncChans2(items)
// 	closedChans := make(map[uint64]struct{})
// 	closedChans[0] = struct{}{}
// 	m := &traceReplay.Machine{threads, nil,
// 		closedChans, make(map[uint64]util.VectorClock),
// 		false,
// 		nil, nil,
// 		make(map[uint64]*util.VarState3),
// 		asyncChans,
// 		make(map[uint64]*util.ChanState),
// 		make([]traceReplay.SelectStore, 0), 1, 1, 0, make([]*util.Item, 0),
// 		make(map[uint64]*util.Item)}
// 	traceReplay.EvListener = []traceReplay.EventListener{
// 		&twinTrack.ListenerSelect{},
// 		&twinTrack.ListenerSync{},
// 		&twinTrack.ListenerAsyncSnd{},
// 		&twinTrack.ListenerAsyncRcv{},
// 		&twinTrack.ListenerChanClose{},
// 		&twinTrack.ListenerOpClosedChan{},
// 		&twinTrack.ListenerDataAccess{},
// 		&twinTrack.ListenerGoStart{},
// 	}
// 	replay(m, json, plain, bench)
// }

// func RunNoName(tracePath string, json, plain, bench bool) {
// 	items := parser.ParseTracev2(tracePath)
// 	// for _, it := range items {
// 	// 	fmt.Println(it)
// 	// }
// 	threads := createThreads(items)

// 	// for _, t := range threads {
// 	// 	fmt.Println(t.ID)
// 	// 	for _, e := range t.Events {
// 	// 		fmt.Printf("\t%v\n", e)
// 	// 	}
// 	// }

// 	asyncChans := getAsyncChans2(items)
// 	closedChans := make(map[uint64]struct{})
// 	closedChans[0] = struct{}{}
// 	m := &traceReplay.Machine{threads, nil,
// 		closedChans, make(map[uint64]util.VectorClock),
// 		false,
// 		nil, nil,
// 		make(map[uint64]*util.VarState3),
// 		asyncChans,
// 		make(map[uint64]*util.ChanState),
// 		make([]traceReplay.SelectStore, 0), 1, 1, 0, make([]*util.Item, 0),
// 		make(map[uint64]*util.Item)}
// 	traceReplay.EvListener = []traceReplay.EventListener{
// 		&graphbuilder.ListenerGoFork{},
// 		&graphbuilder.ListenerGoWait{},
// 		&graphbuilder.ListenerSync{},
// 		&graphbuilder.ListenerAsyncSnd{},
// 		&graphbuilder.ListenerAsyncRcv{},
// 		&graphbuilder.ListenerDataAccess2{},
// 		&noname.ListenerSelect{},
// 		&noname.ListenerSync{},
// 		&noname.ListenerAsyncSnd{},
// 		&noname.ListenerAsyncRcv{},
// 		&noname.ListenerChanClose{},
// 		&noname.ListenerOpClosedChan{},
// 		&noname.ListenerDataAccess2{},
// 		&noname.ListenerGoFork{},
// 		&noname.ListenerGoWait{},
// 	}
// 	replay(m, json, plain, bench)

// 	//print graph
// 	for _, it := range items {
// 		if it.Ops[0].Kind&util.PREPARE > 0 {
// 			continue
// 		}
// 		fmt.Println(it)
// 		for _, x := range it.Next {
// 			fmt.Println("\t", x)
// 		}
// 		fmt.Println("prev", it.Prev)
// 		fmt.Println("----")
// 	}
// 	for _, it := range graphbuilder.EmulatedThreads {
// 		for _, buf := range it.Slots {
// 			for _, ev := range buf.Events {
// 				fmt.Println(ev)
// 				for _, x := range ev.Next {
// 					fmt.Println("\t", x)
// 				}
// 				fmt.Println("-----")
// 			}
// 		}
// 	}

// 	for i, v := range report.UniqueDataRaces {
// 		fmt.Printf("%v\n\t%v\n\t%v\n\n", i, v.A, v.B)
// 		if CheckAlternatives(v.A, v.B, graphbuilder.EmulatedThreads) {
// 			//		if Check(v.A, v.B) {
// 			fmt.Println("Real race!")
// 		} else {
// 			fmt.Println("Fake news!")
// 		}
// 	}

// }
