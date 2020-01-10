package svars

import (
	"fmt"

	"../../util"
	"../analysis"
	"../traceReplay"
)

type ListenerDataAccess struct{}
type PostProcess struct{}

type EventCollector struct{}

var listeners []traceReplay.EventListener

func Init() {
	variables = make(map[uint32]variable)

	listeners = []traceReplay.EventListener{
		&ListenerDataAccess{},
		&PostProcess{},
	}
	algos.RegisterDetector("svars", &EventCollector{})
}

func (l *EventCollector) Put(p *util.SyncPair) {
	for _, l := range listeners {
		l.Put(p)
	}
}

var variables map[uint32]variable

type variable struct {
	thread   uint32
	isShared bool
}

var countAll uint64
var shared uint64

func (l *ListenerDataAccess) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	v, ok := variables[p.T2]
	if !ok {
		countAll++
		v = variable{p.T1, false}
		variables[p.T2] = v
		return
	}

	if v.isShared {
		return
	}

	if p.T1 != v.thread {
		v.isShared = true
		shared++
		variables[p.T2] = v
	}

}

func (l *PostProcess) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}
	fmt.Printf("\nShared:%v/%v\n", shared, countAll)
}
