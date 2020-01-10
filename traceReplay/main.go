package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	race "./detector"

	traceParser "./parser"
	"./util"

	"runtime/pprof"
)

func main() {
	trace := flag.String("trace", "", "path to trace")
	json := flag.Bool("json", false, "output as json")
	plain := flag.Bool("plain", true, "output as plain text")
	bench := flag.Bool("bench", false, "used for benchmarks only")
	analysis := flag.String("mode", "fasttrack", "Use -ls to list available race predictors")
	parser := flag.String("parser", "javainc", "java,javainc")
	repeat := flag.Int("repeat", 2, "multiple runs of the same trace")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
	validateTrace := flag.Bool("validate", false, "set if the trace should be validated for correctness")
	n := flag.Uint64("n", ^uint64(0), "number of events to verify")
	filter := flag.Uint64("filter", 0, "filter level")
	postProcess := flag.Bool("pp", true, "PostProcess")
	ls := flag.Bool("ls", false, "List available race detectors")

	flag.Parse()

	if *ls {
		race.LSDetectors()
		return
	}

	runtime.GOMAXPROCS(4) //might help with the garbage collector, cpus with very high core counts slow it down

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
	//var itemChan chan *util.Item
	if *parser == "go" {
		items = traceParser.ParseTracev2(*trace)
	} else if *parser == "java" || *parser == "java2" {
		items = traceParser.ParseJTracev2(*trace, *n)
	} else {
		//	itemChan = make(chan *util.Item, 100000)
		//	go traceParser.ParseJTracevInc(*trace, *n, itemChan)
	}
	fmt.Println("Parsing trace complete!")

	// for k, v := range traceParser.FileNumToString {
	// 	if v == "NodeSequence.java" {
	// 		fmt.Println(k, "\t", v)
	// 	} else if v == "Lexer.java" {
	// 		fmt.Println(k, "\t", v)
	// 	}
	// }
	// return
	//parser.FileNumToString[loc1.File]

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
	if *parser == "javainc2" {
		race.RunAPISingleIncDouble(*analysis, *postProcess, *trace, *n, *repeat)
	} else if *parser == "java2" {
		race.RunAPISingleDouble(items, *analysis, *filter, *postProcess)
	} else if *parser != "javainc" {
		race.RunAPISingle(items, *analysis, *filter, *postProcess)
	} else {
		race.RunAPISingleInc(*analysis, *postProcess, *trace, *n)
	}

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
