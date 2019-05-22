package report

import (
	"fmt"
	"strings"
	"sync"

	"../../parser"
	"../../util"
	"github.com/fatih/color"
)

const (
	SEVERE = iota << 1
	NORMAL = iota << 1
	LOW    = iota << 1
)

type Level int

var statistics = false

var messageCache map[string]struct{}

func init() {
	messageCache = make(map[string]struct{})
}

func Race(op1, op2 *util.Operation, importance Level) {
	f1 := parser.FileNumToString[op1.SourceRef]
	f2 := parser.FileNumToString[op2.SourceRef]
	s := fmt.Sprintf("Race between:\n1.%v:%v\n2.%v:%v\n\n", f1, op1.Line, f2, op2.Line)

	_, ok := messageCache[s]

	allRaces++
	if ok {
		return
	}
	uniqueRaces++

	s2 := fmt.Sprintf("Race between:\n1.%v:%v\n2.%v:%v\n\n", f2, op2.Line, f1, op1.Line)

	messageCache[s] = struct{}{}
	messageCache[s2] = struct{}{}

	switch importance {
	case SEVERE:
		color.HiRed("\nRace between:\n1.%v:%v\n2.%v:%v\n\n", f1, op1.Line, f2, op2.Line)
	case NORMAL:
		color.HiBlue("\nRace between:\n1.%v:%v\n2.%v:%v\n\n", f1, op1.Line, f2, op2.Line)
	case LOW:
		color.HiGreen("\nRace between:\n1.%v:%v\n2.%v:%v\n\n", f1, op1.Line, f2, op2.Line)
	}
}

type Location struct {
	File uint32
	Line uint32
	W    bool
}

var m = sync.Mutex{}

var raceLoc = make(map[Location]struct{})

var TestFlag bool
var TestFunc func(Location, Location, Level)

func Reset() {
	raceLoc = make(map[Location]struct{})
	messageCache = make(map[string]struct{})
	doubleFilter = make(map[Location]map[Location]struct{})
	doubleFilter2 = make(map[Location]map[Location]struct{})
	allRaces = 0
	uniqueRaces = 0
	writeWriteRace = 0
	writeReadRace = 0
	readWriteRace = 0
	readReadRace = 0
	writeReadDepRace = 0
	missingEdgeCase = 0

	wwFilter = make(map[Location]map[Location]struct{})
	wrFilter = make(map[Location]map[Location]struct{})
	rwFilter = make(map[Location]map[Location]struct{})
	wrdFilter = make(map[Location]map[Location]struct{})
	rrFilter = make(map[Location]map[Location]struct{})

	writeWriteRaceUnique = 0
	writeReadRaceUnique = 0
	readWriteRaceUnique = 0
	readReadRaceUnique = 0
	writeReadDepRaceUnique = 0

	writes = 0
	reads = 0
}

var doubleFilter2 = make(map[Location]map[Location]struct{})

var XXreadReadRace uint64
var XXwriteReadRace uint64
var XXreadWriteRace uint64
var XXwriteWriteRace uint64

func Race4(loc1, loc2 Location, importance Level) {
	allRaces++

	xx, ok := doubleFilter2[loc1]
	if ok {
		if _, ok2 := xx[loc2]; ok2 {
			return
		}
	} else {
		xx = make(map[Location]struct{})
	}
	yy, ok := doubleFilter2[loc2]
	if ok {
		if _, ok2 := yy[loc1]; ok2 {
			return
		}
	} else {
		yy = make(map[Location]struct{})
	}

	xx[loc2] = struct{}{}
	yy[loc1] = struct{}{}
	doubleFilter2[loc1] = xx
	doubleFilter2[loc2] = yy

	uniqueRaces++

	if statistics {
		if loc1.W && loc2.W {
			XXwriteWriteRace++
		} else if loc1.W && !loc2.W {
			XXwriteReadRace++
		} else if !loc1.W && loc2.W {
			XXreadWriteRace++
		} else if !loc1.W && !loc2.W {
			XXreadReadRace++
		}
	}

	f1 := parser.FileNumToString[loc1.File]
	f2 := parser.FileNumToString[loc2.File]
	switch importance {
	case SEVERE:
		sRaces++
		color.HiRed("Race between:\n1.%v:%v\n2.%v:%v\n\n", f1, loc1.Line, f2, loc2.Line)
	case NORMAL:
		nRaces++
		color.HiBlue("Race between:\n1.%v:%v\n2.%v:%v\n\n", f1, loc1.Line, f2, loc2.Line)
	case LOW:
		color.HiGreen("Race between:\n1.%v:%v\n2.%v:%v\n\n", f1, loc1.Line, f2, loc2.Line)
	}
	if TestFlag {
		TestFunc(loc1, loc2, importance)
	}
	ReportNumbers()
}

func Race2(loc1, loc2 Location, importance Level) {
	if importance == NORMAL {
		m.Lock()
		defer m.Unlock()
	}
	raceLoc[loc1] = struct{}{}

	f1 := parser.FileNumToString[loc1.File]
	f2 := parser.FileNumToString[loc2.File]
	s := fmt.Sprintf("Race between:\n1.%v:%v\n2.%v:%v\n\n", f1, loc1.Line, f2, loc2.Line)

	_, ok := messageCache[s]
	allRaces++
	if ok {
		return
	}
	uniqueRaces++
	s2 := fmt.Sprintf("Race between:\n1.%v:%v\n2.%v:%v\n\n", f2, loc2.Line, f1, loc1.Line)

	messageCache[s] = struct{}{}
	messageCache[s2] = struct{}{}

	switch importance {
	case SEVERE:
		sRaces++
		color.HiRed("Race between:\n1.%v:%v\n2.%v:%v\n\n", f1, loc1.Line, f2, loc2.Line)
	case NORMAL:
		nRaces++
		color.HiBlue("Race between:\n1.%v:%v\n2.%v:%v\n\n", f1, loc1.Line, f2, loc2.Line)
	case LOW:
		color.HiGreen("Race between:\n1.%v:%v\n2.%v:%v\n\n", f1, loc1.Line, f2, loc2.Line)
	}
	if TestFlag {
		TestFunc(loc1, loc2, importance)
	}
	ReportNumbers()
}

var allRaces uint64
var uniqueRaces uint64
var sRaces uint64
var nRaces uint64

var reads uint64
var writes uint64
var readReadRace uint64
var writeReadRace uint64
var readWriteRace uint64
var writeWriteRace uint64
var writeReadDep uint64
var writeReadDepRace uint64
var writeReadDepRaceALL uint64

var missingEdgeCase uint64

func MissingEdge() {
	missingEdgeCase++
}

var doubleFilter = make(map[Location]map[Location]struct{})

func ReportRead() {
	reads++
}
func ReportWrite() {
	writes++
}
func RaceStatistics(loc1, loc2 Location, isWrRdDep bool) {
	if isWrRdDep {
		writeReadDepRaceALL++
	}

	xx, ok := doubleFilter[loc1]
	if ok {
		if _, ok2 := xx[loc2]; ok2 {
			return
		}
	} else {
		xx = make(map[Location]struct{})
	}
	yy, ok := doubleFilter[loc2]
	if ok {
		if _, ok2 := yy[loc1]; ok2 {
			return
		}
	} else {
		yy = make(map[Location]struct{})
	}

	xx[loc2] = struct{}{}
	yy[loc1] = struct{}{}
	doubleFilter[loc1] = xx
	doubleFilter[loc2] = yy

	if loc1.W && loc2.W {
		writeWriteRace++
	} else if loc1.W && !loc2.W {
		writeReadRace++
	} else if !loc1.W && loc2.W {
		readWriteRace++
	} else if !loc1.W && !loc2.W {
		readReadRace++
	}

	if isWrRdDep {
		writeReadDepRace++
	}
}

var wwFilter = make(map[Location]map[Location]struct{})
var rrFilter = make(map[Location]map[Location]struct{})
var wrFilter = make(map[Location]map[Location]struct{})
var rwFilter = make(map[Location]map[Location]struct{})
var wrdFilter = make(map[Location]map[Location]struct{})

var readReadRaceUnique uint64
var writeReadRaceUnique uint64
var readWriteRaceUnique uint64
var writeWriteRaceUnique uint64
var writeReadDepUnique uint64
var writeReadDepRaceUnique uint64

var countPhase1 uint64
var countPhase2 uint64
var countPhase1Unique uint64
var countPhase2Unique uint64

// loc1 = prev access, loc2 = current access
// func RaceStatistics2(loc1, loc2 Location, isWrRdDep bool, level int) {
// 	show := false

// 	if level == 0 {
// 		countPhase1++
// 	} else {
// 		countPhase2++
// 	}

// 	if isWrRdDep {
// 		writeReadDepRace++
// 		xx, ok := wrdFilter[loc1]
// 		isNew := true
// 		if ok {
// 			if _, ok2 := xx[loc2]; ok2 {
// 				isNew = false
// 			}
// 		} else {
// 			xx = make(map[Location]struct{})
// 		}
// 		xx[loc2] = struct{}{}
// 		wrdFilter[loc1] = xx

// 		if isNew {
// 			writeReadDepRaceUnique++
// 			color.HiBlue("WRD_RACE:%v, %v\n", loc1, loc2)
// 			show = true
// 		}
// 	}

// 	if loc1.W && loc2.W {
// 		writeWriteRace++
// 		xx, ok := wwFilter[loc1]
// 		if ok {
// 			if _, ok2 := xx[loc2]; ok2 {
// 				return
// 			}
// 		} else {
// 			xx = make(map[Location]struct{})
// 		}
// 		xx[loc2] = struct{}{}
// 		wwFilter[loc1] = xx
// 		writeWriteRaceUnique++
// 		color.HiRed("WWRACE:%v,%v\n", loc1, loc2)
// 		show = true
// 	} else if loc1.W && !loc2.W {
// 		writeReadRace++
// 		// if isWrRdDep {
// 		// 	writeReadDepRace++
// 		// }
// 		xx, ok := wrFilter[loc1]
// 		if ok {
// 			if _, ok2 := xx[loc2]; ok2 {
// 				return
// 			}
// 		} else {
// 			xx = make(map[Location]struct{})
// 		}
// 		xx[loc2] = struct{}{}
// 		wrFilter[loc1] = xx
// 		writeReadRaceUnique++
// 		// if isWrRdDep {
// 		// 	writeReadDepRaceUnique++
// 		// 	color.HiBlue("WRD_RACE:%v, %v\n", loc1, loc2)
// 		// }
// 		color.HiRed("WRRACE:%v,%v\n", loc1, loc2)
// 		show = true
// 	} else if !loc1.W && loc2.W {
// 		readWriteRace++
// 		xx, ok := rwFilter[loc1]
// 		if ok {
// 			if _, ok2 := xx[loc2]; ok2 {
// 				return
// 			}
// 		} else {
// 			xx = make(map[Location]struct{})
// 		}
// 		xx[loc2] = struct{}{}
// 		rwFilter[loc1] = xx
// 		readWriteRaceUnique++
// 		color.HiRed("RWRACE:%v,%v\n", loc1, loc2)
// 		show = true
// 	} else if !loc1.W && !loc2.W {
// 		readReadRace++
// 		xx, ok := rrFilter[loc1]
// 		if ok {
// 			if _, ok2 := xx[loc2]; ok2 {
// 				return
// 			}
// 		} else {
// 			xx = make(map[Location]struct{})
// 		}
// 		xx[loc2] = struct{}{}
// 		rrFilter[loc1] = xx
// 		readReadRaceUnique++
// 	}

// 	if show {
// 		//fmt.Printf("Reads:%v\nWrites:%v\nRead-Read-Races:%v/%v\nRead-Write-Races:%v/%v\nWrite-Read-Races:%v/%v\nWrite-Write-Race:%v/%v\nWrite-Read-Deps:%v\nWrite-Read-Dep-Races:%v/%v\n",
// 		//	reads, writes, readReadRaceUnique, readReadRace, readWriteRaceUnique, readWriteRace, writeReadRaceUnique, writeReadRace, writeWriteRaceUnique, writeWriteRace, writeReadDep, writeReadDepRace, writeReadDepRace)
// 		if level == 0 {
// 			countPhase1Unique++
// 		} else {
// 			countPhase2Unique++
// 		}

// 		fmt.Printf("ALL:%v/%v\n", writeWriteRaceUnique+writeReadRaceUnique+readWriteRaceUnique, writeWriteRace+writeReadRace+readWriteRace)
// 		//ReportNumbers()
// 		if TestFlag {
// 			TestFunc(loc1, loc2, 0)
// 		}
// 	}

// }

var freecounter uint64

func FreeCounter() {
	freecounter++
}

func ReportNumbers() {
	//fmt.Printf("#Races:%v\n#Unique Races:%v\nAt_Runtime:%v\nPostProcessing:%v\n", allRaces, uniqueRaces, sRaces, nRaces)
	fmt.Printf("Reads:%v\nWrites:%v\nRead-Read-Races:%v/%v\nRead-Write-Races:%v/%v\nWrite-Read-Races:%v/%v\nWrite-Write-Race:%v/%v\nWrite-Read-Deps:%v\nWrite-Read-Dep-Races:%v/%v\n",
		reads, writes, readReadRaceUnique, readReadRace, readWriteRaceUnique, readWriteRace, writeReadRaceUnique, writeReadRace,
		writeWriteRaceUnique, writeWriteRace, writeReadDep, writeReadDepRaceUnique, writeReadDepRace)
	fmt.Printf("ALL:%v/%v\n", writeWriteRaceUnique+writeReadRaceUnique+readWriteRaceUnique, writeWriteRace+writeReadRace+readWriteRace)
	fmt.Println("Missing Edge Cases:", missingEdgeCase)
	fmt.Println("Free Counter:", freecounter)
	fmt.Printf("Phase1:%v/%v\n", countPhase1Unique, countPhase1)
	fmt.Printf("Phase2:%v/%v\n", countPhase2Unique, countPhase2)
	//fmt.Println("ALL write-read dep races", writeReadDepRaceALL)
	//if statistics {
	//	fmt.Printf("XXReadRead:%v\nXXRead-Write:%v\nWrite-Read:%v\nWrite-Write:%v\n", XXreadReadRace, XXreadWriteRace, XXwriteReadRace, XXwriteWriteRace)
	//}
}

type DataRace struct {
	A *util.Item
	B *util.Item
}

var UniqueDataRaces []DataRace

func Race3(first, second *util.Item, importance Level) {
	op1 := first.Ops[0]
	op2 := second.Ops[0]
	f1 := parser.FileNumToString[op1.SourceRef]
	f2 := parser.FileNumToString[op2.SourceRef]
	s := fmt.Sprintf("Race between:\n1.%v:%v\n2.%v:%v\n\n", f1, op1.Line, f2, op2.Line)

	_, ok := messageCache[s]

	if ok {
		return
	}

	s2 := fmt.Sprintf("Race between:\n1.%v:%v\n2.%v:%v\n\n", f2, op2.Line, f1, op1.Line)

	messageCache[s] = struct{}{}
	messageCache[s2] = struct{}{}

	UniqueDataRaces = append(UniqueDataRaces, DataRace{first, second})

	switch importance {
	case SEVERE:
		color.HiRed("Race between:\n1.%v:%v\n2.%v:%v\n\n", f1, op1.Line, f2, op2.Line)
	case NORMAL:
		color.HiBlue("Race between:\n1.%v:%v\n2.%v:%v\n\n", f1, op1.Line, f2, op2.Line)
	case LOW:
		color.HiGreen("Race between:\n1.%v:%v\n2.%v:%v\n\n", f1, op1.Line, f2, op2.Line)
	}
}

var alts map[string]map[string]int
var altCount int
var selectAltCount int

func Alternative(op, alt string) {
	if alts == nil {
		alts = make(map[string]map[string]int)
	}

	line := alts[op]
	if line == nil {
		line = make(map[string]int)
	}
	x, ok := line[alt]
	if !ok {
		line[alt] = 1 //new alternative
	} else {
		line[alt] = x + 1
	}
	alts[op] = line
	altCount++
}

func ReportAlts() {

	for k, v := range alts {
		allAcc := 0
		ownAcc := 0
		fmt.Printf("Alternatives for %v:\n", k)
		for x, y := range v {
			fmt.Printf("\t%v, %v\n", x, y)
			allAcc += y
			tmp := strings.Split(k, ":")
			if strings.Contains(x, tmp[0]) {
				ownAcc += y
			}
		}
		fmt.Printf("Ratio:%v/%v\n", ownAcc, allAcc)
		fmt.Printf("\n\n")
	}

	for k, v := range selAlts {
		fmt.Printf("Alternatives for select %v:\n", k)
		for x, y := range v {
			fmt.Printf("\t%v, %v\n", x, y)
		}
		fmt.Printf("\n\n")
	}
}

var selAlts map[string]map[string]int

func SelectAlternative(sel, op string) {
	if selAlts == nil {
		selAlts = make(map[string]map[string]int)
	}

	line := selAlts[sel]
	if line == nil {
		line = make(map[string]int)
	}
	x, ok := line[op]
	if !ok {
		line[op] = 1 //new alternative
	} else {
		line[op] = x + 1
	}
	selAlts[sel] = line
	selectAltCount++
}

type eventPair struct {
	pre  *util.Item
	post *util.Item
}

var EventsList map[uint32][]eventPair

func AddEvent(pre, post *util.Item) {
	return
	if EventsList == nil {
		EventsList = make(map[uint32][]eventPair)
	}
	line := EventsList[pre.Thread]
	line = append(line, eventPair{pre, post})
	EventsList[pre.Thread] = line
}

// func Events() {
// 	for k, v := range EventsList {
// 		fmt.Println(k)
// 		for _, x := range v {
// 			fmt.Printf("\t%v,%v\n", x.pre, x.pre.VC)
// 		}
// 	}
// }

// func AlternativesFromReport() {
// 	count := 0
// 	for k, v := range EventsList {
// 		fmt.Println("Alternatives for thread", k)
// 		for _, a := range v {
// 			alts := alternativesForEvent(a)
// 			count += len(alts)
// 			if len(a.pre.Ops) > 1 {
// 				selectAltCount++
// 			}
// 			idx := findChosenAlt(a, alts)
// 			if len(alts) > 1 {
// 				fmt.Printf("\tFor Event %v\n", a.pre)
// 				for i, b := range alts {
// 					fmt.Printf("\t\t%v-%v\n", i == idx, b.pre)
// 				}
// 			}
// 		}
// 	}
// 	fmt.Println("!!!!!!", "reportalts:", count, "on-fly alts:", altCount, "selalts", selectAltCount)
// }

// func findChosenAlt(it eventPair, alts []eventPair) int {
// 	if it.post == nil {
// 		return -1
// 	}
// 	//sync case
// 	for i, a := range alts {
// 		if a.post != nil && it.post.VC.Equals(a.post.VC) {
// 			return i
// 		}
// 	}
// 	//async case
// 	if it.pre.Ops[0].Kind&util.RCV == 0 {
// 		return 0
// 	}
// 	for i, a := range alts {
// 		if a.post == nil {
// 			continue
// 		}
// 		itVC := it.pre.VC.Clone()
// 		aVC := a.post.VC.Clone()
// 		itVC.Add(it.pre.Thread, 1)
// 		itVC.Sync(aVC)

// 		if itVC.Equals(it.post.VC) {
// 			return i
// 		}
// 	}
// 	return -1
// }

// func alternativesForEvent(it eventPair) []eventPair {
// 	alts := []eventPair{}
// 	for k, v := range EventsList {
// 		if k == it.pre.Thread {
// 			continue
// 		}
// 		for _, a := range v {
// 			if !match(a.pre, it.pre) {
// 				continue
// 			}

// 			if a.pre.VC.ConcurrentTo(it.pre.VC) {
// 				alts = append(alts, a)
// 			}
// 			if it.pre.VC.Less(a.pre.VC) {
// 				break
// 			}
// 		}
// 	}
// 	return alts
// }

func match(it1, it2 *util.Item) bool {
	for _, op1 := range it1.Ops {
		for _, op2 := range it2.Ops {
			if singleMatch(op1, op2) {
				return true
			}
		}
	}
	return false
}

func singleMatch(op1, op2 util.Operation) bool {
	if op1.Ch != op2.Ch {
		return false
	}
	if op1.Kind == op2.Kind {
		return false
	}
	return true
}

var details = true

// loc1 = prev access, loc2 = current access
func RaceStatistics2(loc1, loc2 Location, isWrRdDep bool, level int) {
	show := false

	if level == 0 {
		countPhase1++
	} else {
		countPhase2++
	}

	if isWrRdDep {
		writeReadDepRace++
		xx, ok := wrdFilter[loc1]
		isNew := true
		if ok {
			if _, ok2 := xx[loc2]; ok2 {
				isNew = false
			}
		} else {
			xx = make(map[Location]struct{})
		}
		xx[loc2] = struct{}{}
		wrdFilter[loc1] = xx

		if isNew {
			writeReadDepRaceUnique++
			if !details {
				color.HiBlue("WRD_RACE:%v, %v\n", loc1, loc2)
			} else {
				f1 := parser.FileNumToString[loc1.File]
				f2 := parser.FileNumToString[loc2.File]
				color.HiBlue("WRD_RACE:%v:%v, %v:%v\n", f1, loc1.Line, f2, loc2.Line)
			}
			show = true
		}
	}

	if loc1.W && loc2.W {
		writeWriteRace++
		xx, ok := wwFilter[loc1]
		if ok {
			if _, ok2 := xx[loc2]; ok2 {
				return
			}
		} else {
			xx = make(map[Location]struct{})
		}
		xx[loc2] = struct{}{}
		wwFilter[loc1] = xx
		writeWriteRaceUnique++
		if !details {
			color.HiRed("WWRACE:%v,%v\n", loc1, loc2)
		} else {
			f1 := parser.FileNumToString[loc1.File]
			f2 := parser.FileNumToString[loc2.File]
			color.HiRed("WWRACE:%v:%v, %v:%v\n", f1, loc1.Line, f2, loc2.Line)
		}
		show = true
	} else if loc1.W && !loc2.W {
		writeReadRace++
		// if isWrRdDep {
		// 	writeReadDepRace++
		// }
		xx, ok := wrFilter[loc1]
		if ok {
			if _, ok2 := xx[loc2]; ok2 {
				return
			}
		} else {
			xx = make(map[Location]struct{})
		}
		xx[loc2] = struct{}{}
		wrFilter[loc1] = xx
		writeReadRaceUnique++
		// if isWrRdDep {
		// 	writeReadDepRaceUnique++
		// 	color.HiBlue("WRD_RACE:%v, %v\n", loc1, loc2)
		// }
		if !details {
			color.HiRed("WRRACE:%v,%v\n", loc1, loc2)
		} else {
			f1 := parser.FileNumToString[loc1.File]
			f2 := parser.FileNumToString[loc2.File]
			color.HiRed("WRRACE:%v:%v, %v:%v\n", f1, loc1.Line, f2, loc2.Line)
		}

		show = true
	} else if !loc1.W && loc2.W {
		readWriteRace++
		xx, ok := rwFilter[loc1]
		if ok {
			if _, ok2 := xx[loc2]; ok2 {
				return
			}
		} else {
			xx = make(map[Location]struct{})
		}
		xx[loc2] = struct{}{}
		rwFilter[loc1] = xx
		readWriteRaceUnique++
		if !details {
			color.HiRed("RWRACE:%v,%v\n", loc1, loc2)
		} else {
			f1 := parser.FileNumToString[loc1.File]
			f2 := parser.FileNumToString[loc2.File]
			color.HiRed("RWRACE:%v:%v, %v:%v\n", f1, loc1.Line, f2, loc2.Line)
		}
		show = true
	} else if !loc1.W && !loc2.W {
		readReadRace++
		xx, ok := rrFilter[loc1]
		if ok {
			if _, ok2 := xx[loc2]; ok2 {
				return
			}
		} else {
			xx = make(map[Location]struct{})
		}
		xx[loc2] = struct{}{}
		rrFilter[loc1] = xx
		readReadRaceUnique++
	}

	if show {
		//fmt.Printf("Reads:%v\nWrites:%v\nRead-Read-Races:%v/%v\nRead-Write-Races:%v/%v\nWrite-Read-Races:%v/%v\nWrite-Write-Race:%v/%v\nWrite-Read-Deps:%v\nWrite-Read-Dep-Races:%v/%v\n",
		//	reads, writes, readReadRaceUnique, readReadRace, readWriteRaceUnique, readWriteRace, writeReadRaceUnique, writeReadRace, writeWriteRaceUnique, writeWriteRace, writeReadDep, writeReadDepRace, writeReadDepRace)
		if level == 0 {
			countPhase1Unique++
		} else {
			countPhase2Unique++
		}

		fmt.Printf("ALL:%v/%v\n", writeWriteRaceUnique+writeReadRaceUnique+readWriteRaceUnique, writeWriteRace+writeReadRace+readWriteRace)
		//ReportNumbers()
		if TestFlag {
			TestFunc(loc1, loc2, 0)
		}
	}

}
