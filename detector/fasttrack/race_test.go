package fastTrack

import (
	"fmt"
	"testing"

	"../../util"

	"../report"
)

func TestWLock(t *testing.T) {
	fmt.Println("WLOCK")
	wr1 := util.SyncPair{T1: 1, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 1, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 1}}}}
	wr2 := util.SyncPair{T1: 1, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 1, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 2}}}}
	wr3 := util.SyncPair{T1: 2, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 2, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 3}}}}
	wr4 := util.SyncPair{T1: 3, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 3, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 4}}}}

	lk1 := util.SyncPair{T1: 1, T2: 1, Lock: true, AsyncSend: true}
	uk1 := util.SyncPair{T1: 1, T2: 1, Unlock: true, AsyncRcv: true}
	lk2 := util.SyncPair{T1: 2, T2: 1, Lock: true, AsyncSend: true}
	uk2 := util.SyncPair{T1: 2, T2: 1, Unlock: true, AsyncRcv: true}
	lk3 := util.SyncPair{T1: 1, T2: 1, Lock: true, AsyncSend: true}
	uk3 := util.SyncPair{T1: 1, T2: 1, Unlock: true, AsyncRcv: true}
	lk4 := util.SyncPair{T1: 3, T2: 1, Lock: true, AsyncSend: true}
	uk4 := util.SyncPair{T1: 3, T2: 1, Unlock: true, AsyncRcv: true}

	syncpairs := []util.SyncPair{wr1, wr2,
		lk1, uk1, lk2, uk2, wr3, lk3, uk3, lk4, uk4, wr4}

	Init()
	report.Reset()

	ec := &EventCollector{}
	report.TestFlag = true
	raceCount := 0
	report.TestFunc = func(l1, l2 report.Location, imp report.Level) {
		raceCount++
	}

	for _, s := range syncpairs {
		ec.Put(&s)
	}
	ec.Put(&util.SyncPair{PostProcess: true})
	if raceCount > 1 {
		t.Errorf("Expected one race, got %v!", raceCount)
	}
	fmt.Println("-----------------------------------------------")
}

func TestThreeWrites(t *testing.T) {
	fmt.Println("ThreeWrites")
	wr1 := util.SyncPair{T1: 1, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 1, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 1, Kind: util.WRITE}}}}
	wr2 := util.SyncPair{T1: 1, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 1, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 2, Kind: util.WRITE}}}}
	wr3 := util.SyncPair{T1: 1, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 1, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 3, Kind: util.WRITE}}}}
	wr4 := util.SyncPair{T1: 2, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 2, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 4, Kind: util.WRITE}}}}

	syncpairs := []util.SyncPair{wr1, wr2, wr3, wr4}

	Init()
	report.Reset()

	raceCount := 0
	ec := &EventCollector{}
	report.TestFlag = true

	report.TestFunc = func(l1, l2 report.Location, imp report.Level) {
		raceCount++
		if !(l1.Line == 3 && l2.Line == 4) && !(l1.Line == 2 && l2.Line == 4) && !(l1.Line == 1 && l2.Line == 4) {
			t.Errorf("Wrong Race %v %v %v", l1, l2, imp)
		}
	}

	for _, s := range syncpairs {
		ec.Put(&s)
	}

	ec.Put(&util.SyncPair{PostProcess: true})

	if raceCount != 1 {
		t.Errorf("Expected one race, got %v!", raceCount)
	}
	fmt.Println("-----------------------------------------------")
}

func TestWrdOp(t *testing.T) {
	fmt.Println("WrdOp")
	wr1 := util.SyncPair{T1: 1, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 1, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 1, Kind: util.WRITE}}}}
	wr2 := util.SyncPair{T1: 1, T2: 2, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 1, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 2, Kind: util.WRITE}}}}
	rd1 := util.SyncPair{T1: 2, T2: 2, DataAccess: true, Read: true,
		Ev: &util.Item{Thread: 2, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 3, Kind: util.READ}}}}
	wr3 := util.SyncPair{T1: 2, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 2, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 4, Kind: util.WRITE}}}}
	rd2 := util.SyncPair{T1: 3, T2: 2, DataAccess: true, Read: true,
		Ev: &util.Item{Thread: 3, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 5, Kind: util.READ}}}}
	wr4 := util.SyncPair{T1: 3, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 3, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 6, Kind: util.WRITE}}}}

	syncpairs := []util.SyncPair{wr1, wr2, rd1, wr3, rd2, wr4}

	Init()
	report.Reset()

	raceCount := 0
	ec := &EventCollector{}
	report.TestFlag = true

	report.TestFunc = func(l1, l2 report.Location, imp report.Level) {
		raceCount++
	}

	for _, s := range syncpairs {
		ec.Put(&s)
	}

	ec.Put(&util.SyncPair{PostProcess: true})

	if raceCount != 4 {
		t.Errorf("Expected 4 races, got %v!", raceCount)
	}
	fmt.Println("-----------------------------------------------")
}

func TestRdRdRace(t *testing.T) {
	fmt.Println("RdRdRace")
	wr1 := util.SyncPair{T1: 1, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 1, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 1, Kind: util.WRITE}}}}
	wr2 := util.SyncPair{T1: 1, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 1, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 2, Kind: util.WRITE}}}}
	rd1 := util.SyncPair{T1: 1, T2: 1, DataAccess: true, Read: true,
		Ev: &util.Item{Thread: 1, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 3, Kind: util.READ}}}}
	rd2 := util.SyncPair{T1: 2, T2: 1, DataAccess: true, Read: true,
		Ev: &util.Item{Thread: 2, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 4, Kind: util.READ}}}}

	syncpairs := []util.SyncPair{wr1, wr2, rd1, rd2}

	Init()
	report.Reset()

	raceCount := 0
	ec := &EventCollector{}
	report.TestFlag = true

	report.TestFunc = func(l1, l2 report.Location, imp report.Level) {
		raceCount++
	}

	for _, s := range syncpairs {
		ec.Put(&s)
	}

	ec.Put(&util.SyncPair{PostProcess: true})

	if raceCount != 1 {
		t.Errorf("Expected one race, got %v!", raceCount)
	}
	fmt.Println("-----------------------------------------------")
}

func TestX(t *testing.T) {
	fmt.Println("TESTX")
	wr1 := util.SyncPair{T1: 1, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 1, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 1, Kind: util.WRITE}}}}
	wr2 := util.SyncPair{T1: 1, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 1, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 2, Kind: util.WRITE}}}}
	wr3 := util.SyncPair{T1: 2, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 2, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 3, Kind: util.WRITE}}}}
	wr4 := util.SyncPair{T1: 2, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 2, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 4, Kind: util.WRITE}}}}

	syncpairs := []util.SyncPair{wr1, wr2, wr3, wr4}

	Init()
	report.Reset()

	raceCount := 0
	ec := &EventCollector{}
	report.TestFlag = true

	report.TestFunc = func(l1, l2 report.Location, imp report.Level) {
		raceCount++
		// if imp != 0 {
		// 	t.Errorf("Wrong Race %v %v %v", l1, l2, imp)
		// }
		// if imp == 0 {
		// 	if !(l1.Line == 3 && l2.Line == 2) && !(l1.Line == 5 && l2.Line == 2) && !(l1.Line == 6 && l2.Line == 4) {
		// 		t.Errorf("Wrong Race %v %v %v", l1, l2, imp)
		// 	}
		// }
	}

	for _, s := range syncpairs {
		ec.Put(&s)
	}

	ec.Put(&util.SyncPair{PostProcess: true})

	if raceCount != 1 {
		t.Errorf("Expected one races, got %v!", raceCount)
	}
	fmt.Println("-----------------------------------------------")
}

func TestY(t *testing.T) {
	fmt.Println("TESTY")
	wr1 := util.SyncPair{T1: 1, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 1, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 1}}}}
	wr2 := util.SyncPair{T1: 2, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 2, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 2}}}}
	wr3 := util.SyncPair{T1: 3, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 3, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 3}}}}

	syncpairs := []util.SyncPair{wr1, wr2, wr3}

	Init()
	report.Reset()

	raceCount := 0
	ec := &EventCollector{}
	report.TestFlag = true

	report.TestFunc = func(l1, l2 report.Location, imp report.Level) {
		raceCount++
	}

	for _, s := range syncpairs {
		ec.Put(&s)
	}

	ec.Put(&util.SyncPair{PostProcess: true})

	if raceCount != 2 {
		t.Errorf("Expected two races, got %v!", raceCount)
	}
	fmt.Println("-----------------------------------------------")
}

func TestFirstIsRead(t *testing.T) {
	fmt.Println("FirstIsRead")
	rd1 := util.SyncPair{T1: 1, T2: 1, DataAccess: true, Read: true,
		Ev: &util.Item{Thread: 1, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 1, Kind: util.READ}}}}
	rd2 := util.SyncPair{T1: 2, T2: 1, DataAccess: true, Read: true,
		Ev: &util.Item{Thread: 2, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 2, Kind: util.READ}}}}
	wr2 := util.SyncPair{T1: 2, T2: 1, DataAccess: true, Write: true,
		Ev: &util.Item{Thread: 2, Ops: []util.Operation{util.Operation{SourceRef: 1, Line: 3, Kind: util.WRITE}}}}

	syncpairs := []util.SyncPair{rd1, rd2, wr2}

	Init()
	report.Reset()

	raceCount := 0
	ec := &EventCollector{}
	report.TestFlag = true

	report.TestFunc = func(l1, l2 report.Location, imp report.Level) {
		raceCount++
	}

	for _, s := range syncpairs {
		ec.Put(&s)
	}

	ec.Put(&util.SyncPair{PostProcess: true})

	if raceCount != 1 {
		t.Errorf("Expected three races, got %v!", raceCount)
	}
	fmt.Println("-----------------------------------------------")
}
