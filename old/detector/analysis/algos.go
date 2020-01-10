package algos

import (
	"fmt"

	"../../util"
	"../traceReplay"
)

var detectors map[string]traceReplay.EventListener

func init() {
	detectors = make(map[string]traceReplay.EventListener)
}

func RegisterDetector(name string, d traceReplay.EventListener) {
	detectors[name] = d
}

func GetDetector(name string) traceReplay.EventListener {
	d, ok := detectors[name]
	if !ok {
		panic(fmt.Errorf("unknown detector: %v\n", name))
	}
	return d
}

func PrintRegisteredDetectors() {
	fmt.Println("Available Detectors:")
	for s, _ := range detectors {
		fmt.Println(s)
	}
	fmt.Println("----")
}

type Variable struct {
	T      uint32
	Shared bool
}

var Variables map[uint32]Variable

func FilterUnshared(items []util.Item) {

	Variables = make(map[uint32]Variable)

	for _, it := range items {
		if it.Ops[0].Kind&util.WRITE > 0 || it.Ops[0].Kind&util.READ > 0 || it.Ops[0].Kind&util.ATOMICWRITE > 0 || it.Ops[0].Kind&util.ATOMICREAD > 0 {
			v, ok := Variables[it.Ops[0].Ch]
			if !ok {
				v = Variable{it.Thread, false}
			} else if !v.Shared {
				if v.T != it.Thread {
					v.Shared = true
				}
			}
			Variables[it.Ops[0].Ch] = v
		}
	}

	fmt.Println("All variables:", len(Variables))

	count := 0
	for _, v := range Variables {
		if v.Shared {
			count++
		}
	}

	fmt.Println("Shared variables:", count)
}
