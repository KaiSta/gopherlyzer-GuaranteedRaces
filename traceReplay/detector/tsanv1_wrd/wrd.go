package tsanwrd

import (
	"../../util"
	"../report"
	"github.com/xojoc/bitset"
)

type ListenerDataAccessWRD struct{}

// func (l *ListenerDataAccessWRD) Put(p *util.SyncPair) {
// 	if !p.DataAccess {
// 		return
// 	}

// 	t1, ok := threads[p.T1]
// 	if !ok {
// 		t1 = newT(p.T1)
// 	}

// 	varstate, ok := variables[p.T2]
// 	if !ok {
// 		varstate = newVar()
// 	}

// 	if p.Write {
// 		varstate.lastWrite = t1.vc.clone()
// 	} else if p.Read {
// 		t1.vc = t1.vc.ssync(varstate.lastWrite)
// 	}

// 	threads[p.T1] = t1
// 	variables[p.T2] = varstate
// }

func (l *ListenerDataAccessWRD) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar()
	}

	varstate.current++
	newFE := &dot{v: t1.vc.clone(), int: varstate.current, t: uint16(p.T1),
		sourceRef: p.Ev.Ops[0].SourceRef, line: uint16(p.Ev.Ops[0].Line),
		write: p.Write, ls: make(map[uint32]struct{}), pos: p.Ev.LocalIdx}
	for _, k := range t1.ls { //copy lockset
		newFE.ls[k] = struct{}{}
	}

	if p.Write {
		newFrontier := make([]*dot, 0, len(varstate.frontier))

		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))
			thi_at_j := t1.vc.get(uint32(f.t))

			if k > thi_at_j {
				newFrontier = append(newFrontier, f) // RW(x) =  {j#k | j#k ∈ RW(x) ∧ k > Th(i)[j]}

				if !intersect(newFE.ls, f.ls) {
					report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
						report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
				}

			}
		}

		newFrontier = append(newFrontier, newFE) // ∪{i#Th(i)[i]}
		varstate.frontier = newFrontier
		varstate.lastWrite = t1.vc.clone()
	} else if p.Read {
		newFrontier := make([]*dot, 0, len(varstate.frontier))
		//connectTo := make([]*dot, 0)
		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))          //j#k
			thi_at_j := t1.vc.get(uint32(f.t)) //Th(i)[j]

			if k > thi_at_j {

				newFrontier = append(newFrontier, f) // RW(x) =  {j]k | j]k ∈ RW(x) ∧ k > Th(i)[j]}

				if f.write {
					if !intersect(newFE.ls, f.ls) {
						report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
							report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)
					}
				}
			} else {
				if f.write {
					newFrontier = append(newFrontier, f)
				}
			}
		}
		t1.vc = t1.vc.ssync(varstate.lastWrite)
		newFE.v = t1.vc.clone()
		newFrontier = append(newFrontier, newFE) // ∪{i#Th(i)[i]}
		varstate.frontier = newFrontier
	} else { //volatile synchronize
		vol, ok := volatiles[p.T2]
		if !ok {
			vol = newvc2()
		}
		t1.vc = t1.vc.ssync(vol)
		vol = t1.vc.clone()

		volatiles[p.T2] = vol
	}

	t1.vc = t1.vc.add(p.T1, 1)

	threads[p.T1] = t1
	variables[p.T2] = varstate
}

type ListenerDataAccessWRDEE struct{}

func (l *ListenerDataAccessWRDEE) Put(p *util.SyncPair) {
	if !p.DataAccess {
		return
	}

	t1, ok := threads[p.T1]
	if !ok {
		t1 = newT(p.T1)
	}

	varstate, ok := variables[p.T2]
	if !ok {
		varstate = newVar()
	}

	varstate.current++
	newFE := &dot{v: t1.vc.clone(), int: varstate.current, t: uint16(p.T1),
		sourceRef: p.Ev.Ops[0].SourceRef, line: uint16(p.Ev.Ops[0].Line),
		write: p.Write, ls: make(map[uint32]struct{}), pos: p.Ev.LocalIdx}
	for _, k := range t1.ls { //copy lockset
		newFE.ls[k] = struct{}{}
	}

	newFrontier := make([]*dot, 0, len(varstate.frontier))
	connectTo := make([]*dot, 0)

	if p.Write {

		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))
			thi_at_j := t1.vc.get(uint32(f.t))

			if k > thi_at_j {
				newFrontier = append(newFrontier, f) // RW(x) =  {j#k | j#k ∈ RW(x) ∧ k > Th(i)[j]}

				if !intersect(newFE.ls, f.ls) {
					report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
						report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: true}, false, 0)
				}
				visited := &bitset.BitSet{}
				varstate.findRaces(newFE, f, visited, 0)
			} else {
				connectTo = append(connectTo, f)
			}
		}

		varstate.updateGraph3(newFE, connectTo)

		newFrontier = append(newFrontier, newFE) // ∪{i#Th(i)[i]}
		varstate.frontier = newFrontier

		//connect to artifical start dot if no connection exists
		list, ok := varstate.graph.get(newFE.int)
		if ok && len(list) == 0 {
			list = append(list, &startDot)
			varstate.graph.add(newFE, list)
		}

		varstate.lastWrite = t1.vc.clone()
	} else if p.Read {
		newFE.v = newFE.v.ssync(varstate.lastWrite) //graph needs final vector clocks, we compare against thread vc for wrd races in the loop

		for _, f := range varstate.frontier {
			k := f.v.get(uint32(f.t))          //j#k
			thi_at_j := t1.vc.get(uint32(f.t)) //Th(i)[j]

			if k > thi_at_j {

				newFrontier = append(newFrontier, f) // RW(x) =  {j]k | j]k ∈ RW(x) ∧ k > Th(i)[j]}

				if f.write {
					if !intersect(newFE.ls, f.ls) {
						report.ReportRace(report.Location{File: uint32(f.sourceRef), Line: uint32(f.line), W: f.write},
							report.Location{File: p.Ev.Ops[0].SourceRef, Line: p.Ev.Ops[0].Line, W: newFE.write}, false, 0)
					}
					visited := &bitset.BitSet{}
					varstate.findRaces(newFE, f, visited, 0)
				}
			} else {
				if f.int > 0 {
					connectTo = append(connectTo, f)
				}
				if f.write {
					newFrontier = append(newFrontier, f)
				}
			}
		}

		varstate.updateGraph3(newFE, connectTo)

		newFrontier = append(newFrontier, newFE) // ∪{i#Th(i)[i]}
		varstate.frontier = newFrontier

		//connect to artifical start dot if no connection exists
		list, ok := varstate.graph.get(newFE.int)
		if ok && len(list) == 0 {
			list = append(list, &startDot)
			varstate.graph.add(newFE, list)
		}

		t1.vc = t1.vc.ssync(varstate.lastWrite)
	} else { //volatile synchronize
		vol, ok := volatiles[p.T2]
		if !ok {
			vol = newvc2()
		}
		t1.vc = t1.vc.ssync(vol)
		vol = t1.vc.clone()

		volatiles[p.T2] = vol
	}

	t1.vc = t1.vc.add(p.T1, 1)

	threads[p.T1] = t1
	variables[p.T2] = varstate
}
