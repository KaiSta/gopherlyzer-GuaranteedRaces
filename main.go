package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"./detector"

	traceParser "./parser"
	"./util"

	"runtime/pprof"
)

func main() {
	trace := flag.String("trace", "", "path to trace")
	json := flag.Bool("json", false, "output as json")
	plain := flag.Bool("plain", true, "output as plain text")
	bench := flag.Bool("bench", false, "used for benchmarks only")
	analysis := flag.String("mode", "fasttrack", "select between eraser,racetrack,fasttrack,tsan")
	parser := flag.String("parser", "go", "go or java")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
	validateTrace := flag.Bool("validate", false, "set if the trace should be validated for correctness")
	n := flag.Uint64("n", ^uint64(0), "number of events to verify")
	filter := flag.Uint64("filter", 0, "filter level")
	postProcess := flag.Bool("pp", true, "PostProcess")
	ls := flag.Bool("ls", false, "List available race detectors")

	flag.Parse()

	if *ls {
		race.LSDetectors()
	}

	runtime.GOMAXPROCS(4)

	var f *os.File

	if !*json && !*plain && !*bench {
		panic("no output format defined")
	}

	if trace == nil || *trace == "" {
		panic("no valid trace file")
	}
	if *analysis == "" {
		panic("no analysis mode chosen")
	}

	fmt.Println("Parsing trace...")
	a, b := filepath.Abs(*trace)
	fmt.Println("Trace path:", a, b)
	var items []util.Item
	if *parser == "go" {
		items = traceParser.ParseTracev2(*trace)
	} else {
		items = traceParser.ParseJTracev2(*trace, *n)
	}
	fmt.Println("Parsing trace complete!")

	if *validateTrace {
		fmt.Println("Validating trace...")
		//util.ValidateTrace(items)
		race.RunAPIV2(items, *analysis, *filter)
		fmt.Println("Validating successful")
	}

	if *cpuprofile != "" {
		f, _ = os.Create(*cpuprofile)
		pprof.StartCPUProfile(f)
	}

	//debug.SetGCPercent(-1)

	race.RunAPISingle(items, *analysis, *filter, *postProcess)
	if *cpuprofile != "" {
		pprof.StopCPUProfile()
	}

	return
	// if *plain {
	// 	color.HiGreen("Covered schedules")
	// 	color.HiRed("Uncovered schedules")
	// 	color.HiYellow("-----------------------")
	// }

	//	race.Run(*trace, *json, *plain, *bench)
	// if *parser == "go" {
	// 	if *analysis == "fasttrack" {
	// 		race.RunFastTrack(*trace, *json, *plain, *bench)
	// 	} else if *analysis == "racetrack" {
	// 		race.RunRaceTrack(*trace, *json, *plain, *bench)
	// 	} else if *analysis == "eraser" {
	// 		race.RunEraser(*trace, *json, *plain, *bench)
	// 	} else if *analysis == "twintrack" {
	// 		race.RunTwinTrack(*trace, *json, *plain, *bench)
	// 	} else if *analysis == "tsan" {
	// 		race.RunThreadSanitizer(*trace, *json, *plain, *bench)
	// 	} else if *analysis == "noname" {
	// 		race.RunNoName(*trace, *json, *plain, *bench)
	// 	} else if *analysis == "gotrack" {
	// 		race.RunGoTrack(*trace, *json, *plain, *bench)
	// 	}
	// } else {

	// }
}
