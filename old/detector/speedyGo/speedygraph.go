package speedygo

import (
	"fmt"

	"../report"
)

type graph struct {
	head *node
}

type node struct {
	first dot
	last  dot
	next  map[*node]struct{}
	prev  map[*node]struct{}
}

func newGraph() *graph {
	return &graph{&node{0, 0, make(map[*node]struct{}), make(map[*node]struct{})}}
}
func (g *graph) add(d dot, connections versionVector) {
	newNode := &node{d, d, make(map[*node]struct{}), make(map[*node]struct{})}
	//g.add2(g.head, newNode, connections)
	g.compress(newNode)
}

// func (g *graph) add2(current *node, newNode *node, connections versionVector) {

// 	if len(connections) == 0 {
// 		g.head.prev[newNode] = struct{}{}
// 		newNode.next[g.head] = struct{}{}
// 		return
// 	}

// 	for s := range connections {
// 		if current.first <= s && current.last > s { // new node must connect to something in the middle
// 			//split range node into three parts, connect new node to the middle part

// 			if current.last-current.first > 1 { // current node contains at least three versions
// 				leftNode := &node{first: current.first, last: s - 1}
// 				middleNode := &node{first: s, last: s}
// 				rightNode := &node{first: s + 1, last: current.last}

// 				leftNode.next = current.next
// 				for n := range leftNode.next {
// 					delete(n.prev, current)
// 					n.prev[leftNode] = struct{}{}
// 				}
// 				leftNode.prev = map[*node]struct{}{middleNode: struct{}{}}

// 				middleNode.next = map[*node]struct{}{leftNode: struct{}{}}
// 				middleNode.prev = map[*node]struct{}{rightNode: struct{}{}}

// 				rightNode.next = map[*node]struct{}{middleNode: struct{}{}}
// 				rightNode.prev = current.prev
// 				for n := range rightNode.prev {
// 					delete(n.next, current)
// 					n.next[rightNode] = struct{}{}
// 				}

// 				middleNode.prev[newNode] = struct{}{}
// 				newNode.next[middleNode] = struct{}{}
// 			} else {
// 				leftNode := &node{first: current.first, last: current.first}
// 				rightNode := &node{first: current.last, last: current.last}

// 				leftNode.next = current.next
// 				for n := range leftNode.next {
// 					delete(n.prev, current)
// 					n.prev[leftNode] = struct{}{}
// 				}
// 				leftNode.prev = map[*node]struct{}{rightNode: struct{}{}}

// 				rightNode.next = map[*node]struct{}{leftNode: struct{}{}}
// 				rightNode.prev = current.prev
// 				for n := range rightNode.prev {
// 					delete(n.next, current)
// 					n.next[rightNode] = struct{}{}
// 				}

// 				if s == leftNode.first {
// 					leftNode.prev[newNode] = struct{}{}
// 					newNode.next[leftNode] = struct{}{}
// 				} else {
// 					rightNode.prev[newNode] = struct{}{}
// 					newNode.next[rightNode] = struct{}{}
// 				}
// 			}

// 		} else if current.last == s {
// 			current.prev[newNode] = struct{}{}
// 			newNode.next[current] = struct{}{}
// 		}
// 	}

// 	for p := range current.prev {
// 		g.add2(p, newNode, connections)
// 	}
// }

func (g *graph) compress(n *node) {
	if len(n.next) != 1 {
		return
	}

	for p := range n.next { //just one node, loop makes it easier to access the single node
		if p.last != 0 && (p.last+1) == n.first {
			delete(p.prev, n)
			p.last++
		}
	}
}

func (g *graph) search(n *node, d dot) (bool, map[dot]struct{}) {
	path := make(map[dot]struct{})
	finish := false
	for i := n.first; i <= n.last; i++ {
		if i == d {
			finish = true
		} else {
			path[i] = struct{}{}
		}

	}
	if finish {
		return true, path
	}

	if len(n.prev) == 0 {
		return false, path
	}

	for x := range n.prev {
		if ok, p := g.search(x, d); ok {
			for k := range p {
				path[k] = struct{}{}
			}
			return true, path
		}
	}

	return false, nil
}

func (n *node) String() string {
	col := ""
	for x := range n.next {
		if col != "" {
			col += ","
		}
		col += fmt.Sprintf("(%v,%v)", x.first, x.last)
	}

	return fmt.Sprintf("(%v,%v,[%v])", n.first, n.last, col)
}

func (g *graph) print(n *node, dfilter map[*node]struct{}) string {
	if n == nil {
		return ""
	}

	s := n.String()

	for p := range n.prev {
		tmp := g.print(p, dfilter)
		if _, ok := dfilter[p]; !ok {
			s += fmt.Sprintf("\n%v", tmp)
			dfilter[p] = struct{}{}
		}
	}
	return s
}
func updateGraph(nv dot, vv versionVector, v *variable) {
	v.history.add(nv, vv)
}
func detectGraphRaces(raceAcc dot, racePartner []dot, v *variable) {
	ok, reachRaceAcc := v.history.search(v.history.head, raceAcc)

	if !ok {
		panic("race detection for unknown version!")
	}

	for _, rp := range racePartner {
		ok, currentReach := v.history.search(v.history.head, rp)

		if !ok {
			panic("race detection for unknown version! 2")
		}

		for s := range currentReach {
			if _, ok := reachRaceAcc[s]; !ok {
				if v.versionInfo.Get(int(raceAcc)) || v.versionInfo.Get(int(s)) {
					ev := v.versionLoc[raceAcc]
					ev1 := v.versionLoc[s]
					report.Race2(ev1, ev, report.SEVERE)
				}
			}
		}
	}
}

func updateGraph2(nv dot, vv versionVector, vari *variable) {
	col := vari.history2[nv]
	col.c = false
	if col.x == nil {
		col.x = newVV()
	}
	col.x = vv.clone()

	if debug {
		fmt.Printf("Reach of %v: %v\n", nv, col.x)
	}

	vari.history2[nv] = col

	//col.x = col.x.union(calcReach(nv, vari.history2))
	//col.c = true
	//vari.history2[nv] = col

}
func detectGraphRaces2(raceAcc dot, racePartner []dot, vari *variable) {
	// if len(racePartner) == 0 {
	// 	return
	// }

	reachRaceAcc := calcReach(raceAcc, vari.history2)
	if debug {
		fmt.Println("Reach1:", reachRaceAcc)
	}
	h := vari.history2[raceAcc]
	if !h.c {
		h.x = reachRaceAcc
		h.c = true
		vari.history2[raceAcc] = h
	}

	for _, rp := range racePartner {
		currentReach := calcReach(rp, vari.history2)
		if debug {
			fmt.Println("Reach2:", currentReach)
		}
		h := vari.history2[rp]
		if !h.c {
			h.x = currentReach
			h.c = true
			vari.history2[rp] = h
		}

		for s := range currentReach {
			if !reachRaceAcc.contains(s) {
				if vari.versionInfo.Get(int(raceAcc)) || vari.versionInfo.Get(int(s)) {
					ev := vari.versionLoc[raceAcc]
					ev1 := vari.versionLoc[s]
					report.Race2(ev1, ev,
						report.SEVERE)
				}
			}
		}
	}
}

func calcReach(current dot, history map[dot]valHistory) versionVector {
	h := history[current]
	if h.c {
		return h.x
	}
	result := newVV()
	for d := range h.x { //copy reach of current dot
		result.add(d)
		currentReach := calcReach(d, history)
		h := history[d]
		if !h.c {
			h.x = currentReach
			h.c = true
			history[d] = h
		}
		result = result.union(currentReach)
	}
	//fmt.Println(result)
	return result
	// for k := range result {
	// 	h.x = append(h.x, k)
	// }
	// h.c = true
}

type valHistory struct {
	x versionVector
	c bool
}
