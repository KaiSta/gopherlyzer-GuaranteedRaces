package sshb

import (
	"fmt"

	"../../util"
	"../analysis"
	"../report"
	"../traceReplay"
)

type ListenerAsyncSnd struct{}
type ListenerAsyncRcv struct{}
type ListenerSync struct{}
type ListenerDataAccess struct{}
type ListenerGoFork struct{}
type ListenerGoWait struct{}
type ListenerPostProcess struct{}
type ListenerPostProcessLarge struct{}

type EventCollector struct {
	listeners []traceReplay.EventListener
}

var statistics = true
var doPostProcess = true

func Init() {
	threads = make(map[uint32]thread)
	locks = make(map[uint32]lock)
	signalList = make(map[uint32]signal)
	variables = make(map[uint32]variable)

	listeners1 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccess{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerPostProcess{},
	}
	algos.RegisterDetector("sshb", &EventCollector{listeners1})

	listeners2 := []traceReplay.EventListener{
		&ListenerAsyncSnd{},
		&ListenerAsyncRcv{},
		&ListenerSync{},
		&ListenerDataAccess{},
		&ListenerGoFork{},
		&ListenerGoWait{},
		&ListenerPostProcessLarge{},
	}
	algos.RegisterDetector("sshbLarge", &EventCollector{listeners2})
}

var threads map[uint32]thread
var locks map[uint32]lock
var signalList map[uint32]signal
var variables map[uint32]variable

type thread struct {
	vc    vcepoch
	curr  *node
	first *node
	ls    map[uint32]struct{}
}

func newThread(tid uint32) thread {
	return thread{newvc2().set(tid, 1), nil, nil, make(map[uint32]struct{})}
}

var threads2 = make(map[uint32][]*node)

type lock struct {
	vc   vcepoch
	curr *node
}

type signal struct {
	vc   vcepoch
	curr *node
}

type variable struct {
	lastWrite     vcepoch
	lwEv          *util.Item
	rvc           vcepoch
	wvc           vcepoch
	lastEv        *util.Item
	hasRace       bool
	writes        []*node
	reads         []*node
	races         []race
	lastWriteNode *node
}

var nodes []*node

type race struct {
	acc1    *node
	acc2    *node
	lsEmpty bool
}

type variableHistory struct {
	ev    *util.Item
	clock vcepoch
}

type node struct {
	ev      *util.Item
	next    []*node
	clock   vcepoch
	visited bool
	ls      map[uint32]struct{}
}

func newVar() variable {
	return variable{newvc2(), nil, newEpoch(0, 0), newEpoch(0, 0),
		nil, false, make([]*node, 0), make([]*node, 0), make([]race, 0), nil}
}

func (l *EventCollector) Put(p *util.SyncPair) {
	//	syncPairTrace = append(syncPairTrace, p)
	for _, l := range l.listeners {
		l.Put(p)
	}
}

var uniqueRaceFilter = make(map[string]map[string]struct{})

func isUnique(r race) bool {
	s1 := fmt.Sprintf("f:%vl%v", r.acc1.ev.Ops[0].SourceRef, r.acc1.ev.Ops[0].Line)
	s2 := fmt.Sprintf("f:%vl%v", r.acc2.ev.Ops[0].SourceRef, r.acc2.ev.Ops[0].Line)

	s1map := uniqueRaceFilter[s1]
	if s1map == nil {
		s1map = make(map[string]struct{})
	}

	if _, ok := s1map[s2]; !ok {
		s1map[s2] = struct{}{}
		s2map := uniqueRaceFilter[s2]
		if s2map == nil {
			s2map = make(map[string]struct{})
		}
		s2map[s1] = struct{}{}
		uniqueRaceFilter[s1] = s1map
		uniqueRaceFilter[s2] = s2map
		return true
	}
	return false
}

func intersect(a, b map[uint32]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; ok {
			return true
		}
	}
	return false
}

func (l *ListenerDataAccess) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread(p.T1)
	}

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar()
	}

	newNode := &node{ev: p.Ev, clock: t1.vc.clone(), next: make([]*node, 0), ls: make(map[uint32]struct{})}
	for k := range t1.ls {
		newNode.ls[k] = struct{}{}
	}
	//Program order
	if t1.curr != nil {
		t1.curr.next = append(t1.curr.next, newNode)
	}
	nodes = append(nodes, newNode)
	t1.curr = newNode
	list := threads2[p.T1]
	list = append(list, newNode)
	threads2[p.T1] = list

	if p.Write {
		newWrites := make([]*node, 0)
		if !varstate.wvc.leq(t1.vc) {
			//concurrent writes exist
			for i, w := range varstate.writes {
				k := w.clock.get(w.ev.Thread)
				curr := t1.vc.get(w.ev.Thread)
				if k > curr {
					newWrites = append(newWrites, varstate.writes[i])
					r := race{varstate.writes[i], newNode, !intersect(t1.ls, w.ls)}
					if isUnique(r) {
						varstate.races = append(varstate.races, r)
					}
				}
			}
		}
		newWrites = append(newWrites, newNode)
		varstate.wvc = varstate.wvc.set(p.T1, t1.vc.get(p.T1))

		if !varstate.rvc.leq(t1.vc) {
			//concurrent reads exist
			for i, r := range varstate.reads {
				k := r.clock.get(r.ev.Thread)
				curr := t1.vc.get(r.ev.Thread)
				if k > curr {
					//update wrd graph
					newNode.next = append(newNode.next, varstate.reads[i])
					//store race
					r := race{varstate.reads[i], newNode, !intersect(t1.ls, r.ls)}
					if isUnique(r) {
						varstate.races = append(varstate.races, r)
					}
				}
			}
		}

		varstate.writes = newWrites
		t1.vc = t1.vc.add(p.T1, 1)
	} else if p.Read {
		//find concurrent writes, add wrd and store the detected race
		if !varstate.wvc.leq(t1.vc) {
			for i, w := range varstate.writes {
				k := w.clock.get(w.ev.Thread)
				curr := t1.vc.get(w.ev.Thread)
				if k > curr {
					varstate.writes[i].next = append(varstate.writes[i].next, newNode)
					r := race{varstate.writes[i], newNode, !intersect(t1.ls, w.ls)}
					if isUnique(r) {
						varstate.races = append(varstate.races, r)
					}
				}
			}
		}

		newReads := make([]*node, 0)
		if !varstate.rvc.leq(t1.vc) {
			for i, r := range varstate.reads {
				k := r.clock.get(r.ev.Thread)
				curr := t1.vc.get(r.ev.Thread)
				if k > curr {
					newReads = append(newReads, varstate.reads[i])
				}
			}
		}
		newReads = append(newReads, newNode)

		varstate.rvc = varstate.rvc.set(p.T1, t1.vc.get(p.T1))
		varstate.reads = newReads
		t1.vc = t1.vc.add(p.T1, 1)
	}

	varstate.lastEv = p.Ev
	variables[p.T2] = varstate
	threads[p.T1] = t1
}

func (l *ListenerAsyncSnd) Put(p *util.SyncPair) {
	if !p.AsyncSend {
		return
	}

	if !p.Lock {
		return
	}

	lock, ok := locks[p.T2]
	if !ok {
		lock.vc = newvc2()
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread(p.T1)
	}

	t1.vc = t1.vc.ssync(lock.vc)

	//Program order
	newNode := &node{ev: p.Ev, clock: t1.vc.clone(), next: make([]*node, 0)}
	if t1.curr != nil { //first node
		t1.curr.next = append(t1.curr.next, newNode)
	}
	t1.curr = newNode
	nodes = append(nodes, newNode)
	list := threads2[p.T1]
	list = append(list, newNode)
	threads2[p.T1] = list

	//RAD order
	if lock.curr != nil {
		lock.curr.next = append(lock.curr.next, newNode)
	}

	t1.ls[p.T2] = struct{}{}

	threads[p.T1] = t1
}

func (l *ListenerAsyncRcv) Put(p *util.SyncPair) {
	if !p.AsyncRcv {
		return
	}

	if !p.Unlock {
		return
	}

	lock, ok := locks[p.T2]
	if !ok {
		lock.vc = newvc2()
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread(p.T1)
	}

	//Program order
	newNode := &node{ev: p.Ev, clock: t1.vc.clone(), next: make([]*node, 0)}
	if t1.curr != nil { //first node
		t1.curr.next = append(t1.curr.next, newNode)
	}
	t1.curr = newNode
	nodes = append(nodes, newNode)
	list := threads2[p.T1]
	list = append(list, newNode)
	threads2[p.T1] = list

	//RAD order
	lock.curr = newNode

	lock.vc = t1.vc.clone()
	t1.vc = t1.vc.add(p.T1, 1)

	delete(t1.ls, p.T2)

	threads[p.T1] = t1
	locks[p.T2] = lock
}

func (l *ListenerSync) Put(p *util.SyncPair) {
	if !p.Sync {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread(p.T1)
	}
	t2, ok := threads[p.T2]
	if !ok {
		t2 = newThread(p.T2)
	}

	t1.vc = t1.vc.add(p.T1, 1)
	t2.vc = t2.vc.add(p.T2, 1)

	t1.vc = t1.vc.ssync(t2.vc)

	threads[p.T1] = t1
	threads[p.T2] = t2
}

func (l *ListenerGoFork) Put(p *util.SyncPair) {
	if !p.IsFork { //used for sig - wait too
		return
	}
	t1, ok := threads[p.T1]
	if !ok {
		t1 = newThread(p.T1)
	}

	//Program order
	newNode := &node{ev: p.Ev, clock: t1.vc.clone(), next: make([]*node, 0)}
	if t1.curr != nil { //first node
		t1.curr.next = append(t1.curr.next, newNode)
	}
	t1.curr = newNode
	nodes = append(nodes, newNode)
	list := threads2[p.T1]
	list = append(list, newNode)
	threads2[p.T1] = list

	signalList[p.T2] = signal{t1.vc.clone(), newNode}

	t1.vc = t1.vc.add(p.T1, 1)

	threads[p.T1] = t1
}

func (l *ListenerGoWait) Put(p *util.SyncPair) {
	if !p.IsWait {
		return
	}

	vc, ok := signalList[p.T2]

	if ok {
		t1, ok := threads[p.T1]
		if !ok {
			t1 = newThread(p.T1)
		}
		t1.vc = t1.vc.ssync(vc.vc)
		t1.vc = t1.vc.add(p.T1, 1)

		//Program order
		newNode := &node{ev: p.Ev, clock: t1.vc.clone(), next: make([]*node, 0)}
		if t1.curr != nil { //first node
			t1.curr.next = append(t1.curr.next, newNode)
		}
		t1.curr = newNode
		nodes = append(nodes, newNode)
		list := threads2[p.T1]
		list = append(list, newNode)
		threads2[p.T1] = list

		//Fork order
		vc.curr.next = append(vc.curr.next, newNode)

		threads[p.T1] = t1
	}
}

func (l *ListenerPostProcess) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}

	fmt.Println("Post processing starts...")
	sumRaces := 0
	for _, v := range variables {
		sumRaces += len(v.races)
	}
	fmt.Printf("Variables:%v\nDynamicRaces:%v\nUniqueRaces:%v\n", len(variables), sumRaces, sumRaces)

	falsePositives := 0

	steps := 0
	complete := sumRaces
	stepBarrier := (complete / 20) + 1

	for _, v := range variables {
		for _, r := range v.races {
			// check the race if its valide
			if !checkRaceValidity(r) {
				report.RaceStatistics2(
					report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
					report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
					false, 0)
				if r.lsEmpty {
					falsePositives++
				}
			}
			steps++
			if steps%stepBarrier == 0 {
				fmt.Printf("\r%v/%v", steps, complete)
			}
		}
	}
	fmt.Println("FALSE POSITIVES:", falsePositives)

	countAlternativeWrites()
}

func (l *ListenerPostProcessLarge) Put(p *util.SyncPair) {
	if !p.PostProcess {
		return
	}

	fmt.Println("Post processing starts...")
	sumRaces := 0
	for _, v := range variables {
		sumRaces += len(v.races)
	}
	fmt.Printf("Variables:%v\nDynamicRaces:%v\nUniqueRaces:%v\n", len(variables), sumRaces, sumRaces)

	steps := 0
	complete := sumRaces
	stepBarrier := (complete / 20) + 1

	for _, v := range variables {
		for _, r := range v.races {
			// check the race if its valide
			if !checkRaceValidityLarge(r) {
				report.RaceStatistics2(
					report.Location{File: r.acc1.ev.Ops[0].SourceRef, Line: r.acc1.ev.Ops[0].Line, W: r.acc1.ev.Ops[0].Kind&util.WRITE > 0},
					report.Location{File: r.acc2.ev.Ops[0].SourceRef, Line: r.acc2.ev.Ops[0].Line, W: r.acc2.ev.Ops[0].Kind&util.WRITE > 0},
					false, 0)
			}
			steps++
			if steps%stepBarrier == 0 {
				fmt.Printf("\r%v/%v", steps, complete)
			}
		}
	}
	fmt.Println("Sum Races", sumRaces)
}

func checkRaceValidity(r race) bool {
	valid, _ := search(r.acc1, r.acc2, 0)
	for i := range nodes {
		nodes[i].visited = false
	}

	return valid
}
func checkRaceValidityLarge(r race) bool {
	valid, _ := dsearch(r.acc1, r.acc2, 0)
	for i := range nodes {
		nodes[i].visited = false
	}

	return valid
}

func trivialPathExists(r race) bool {
	for _, c := range r.acc1.next {
		if c == r.acc2 {
			return true
		}
	}
	return false
}

var reachMap = make(map[*node][]*node)

func search(current, finish *node, level int) (bool, int) {

	if level > 100000 {
		return dsearch(current, finish, level)
	}

	current.visited = true

	k := finish.clock.get(finish.ev.Thread)
	curr := current.clock.get(finish.ev.Thread)
	if k < curr {
		return false, level
	}

	if current == finish { //found a way
		return true, level
	}

	if len(current.next) == 0 { //no child nodes
		return false, level
	}

	//breadth part
	for i := range current.next {
		if level != 0 && current.next[i] == finish {
			return true, level
		}
	}

	for i := range current.next {
		//if _, f := visited[current.next[i]]; !f {
		if !current.next[i].visited {
			if !(level == 0 && current.next[i] == finish) {
				if ok, el := search(current.next[i], finish, level+1); ok { //external links
					// if level == 0 {
					// 	fmt.Println("!!!!!!", el)
					// }
					return true, el // append(path, p...)
				}
			}
		}
		//}
	}

	return false, level
}

func bsearch(start, finish *node, level int) (bool, int) {
	if start == finish {
		return true, 0
	}

	queue := make([]*node, 0)
	queue = append(queue, start)

	var current *node

	for len(queue) != 0 {
		current, queue = queue[0], queue[1:]
		if current == finish {
			return true, 0
		}
		if len(current.next) == 0 {
			return false, 0
		}
		queue = append(queue, current.next...)
	}

	return false, 0
}

func dsearch(start, finish *node, level int) (bool, int) {
	stack := make([]*node, 0, 50000)
	stack = append(stack, start)
	var curr *node

	k := finish.clock.get(finish.ev.Thread)

	for len(stack) != 0 {
		curr, stack = stack[len(stack)-1], stack[:len(stack)-1]

		if !curr.visited {
			curr.visited = true
		}

		if curr == finish {
			return true, 0
		}

		for i := range curr.next {
			if !curr.next[i].visited && !(level == 0 && curr.next[i] == finish) { //level==0 filters the trivial path
				nextClock := curr.next[i].clock.get(finish.ev.Thread)
				if !(k < nextClock) {
					stack = append(stack, curr.next[i])
				}
			}
		}
		level++
	}

	return false, 0
}

var readCounts = make(map[*node]uint)

func countAlternativeWrites() {

	for _, t := range threads2 {
		for _, x := range t {
			if x.ev.Ops[0].Kind&util.WRITE > 0 {
				for _, n := range x.next {
					if n.ev.Ops[0].Kind&util.READ > 0 {
						val := readCounts[n]
						val++
						readCounts[n] = val
					}
				}
			}
		}
	}

	sum := uint(0)
	max := uint(0)
	for _, v := range readCounts {
		sum += v
		if v > max {
			max = v
		}
	}
	fmt.Printf("Max writes:%v\nAvg writes:%v\n", max, float64(sum)/float64(len(readCounts)))
}
