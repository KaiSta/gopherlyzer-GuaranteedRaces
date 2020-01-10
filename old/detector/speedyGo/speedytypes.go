package speedygo

import (
	"../report"
	"github.com/xojoc/bitset"
)

type variable struct {
	current     uint
	dots        []dot
	lastWrite   varset
	lwDot       dot
	lwT         uint64
	history     *graph
	history2    map[dot]valHistory
	versionLoc  map[dot]report.Location
	versionInfo *bitset.BitSet
}

func newVar() variable {
	//return variable{1, make([]dot, 0, 4), newVarSet(), dot(0), newGraph(), make(map[dot]report.Location), &bitset.BitSet{}}
	return variable{1, make([]dot, 0, 4), newVarSet(), dot(0), 0, newGraph(), make(map[dot]valHistory), make(map[dot]report.Location), &bitset.BitSet{}}
}

var threads map[uint64]varset
var locks map[uint64]varset
var signalList map[uint64]varset
var variables map[uint64]variable
var threadLWs map[uint64]varset

type varset map[uint64]versionVector

func newVarSet() varset {
	return make(map[uint64]versionVector)
}
func (v varset) union(v2 varset) varset {
	for k, vv := range v2 {
		vv3, ok := v[k]
		if !ok {
			vv3 = newVV()
		}
		vv3 = vv3.union(vv)
		v[k] = vv3
	}
	return v
}

func (v varset) clone() varset {
	v2 := newVarSet()
	for k, vv := range v {
		v2[k] = vv.clone()
	}
	return v2
}

// type versionVector []dot

// func newVV() versionVector {
// 	return make([]dot, 0, 16)
// }
// func (v versionVector) clone() versionVector {
// 	return v.union(newVV())
// }
// func (v versionVector) union(v2 versionVector) versionVector {
// 	v = append(v, v2...)
// 	return v
// }
// func (v versionVector) compress() versionVector {
// 	if len(v) > 100 {
// 		v3 := make(map[dot]struct{})
// 		for _, x := range v {
// 			v3[x] = struct{}{}
// 		}
// 		v = newVV()
// 		for k := range v3 {
// 			v = append(v, k)
// 		}
// 	}
// 	return v
// }
// func (v versionVector) add(d dot) {
// 	v = append(v, d)
// }

// func (v versionVector) contains(d dot) bool {
// 	for _, x := range v {
// 		if x == d {
// 			return true
// 		}
// 	}
// 	return false
// }

type versionVector map[dot]struct{}

func newVV() versionVector {
	return make(map[dot]struct{})
}
func (v versionVector) clone() versionVector {
	return v.union(newVV())
}
func (v versionVector) union(v2 versionVector) versionVector {
	for s := range v2 {
		v[s] = struct{}{}
	}
	return v
}
func (v versionVector) contains(d dot) bool {
	_, ok := v[d]
	return ok
}
func (v versionVector) add(d dot) {
	v[d] = struct{}{}
}

type dot uint
