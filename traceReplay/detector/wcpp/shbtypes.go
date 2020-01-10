package wcpp

import "fmt"

type epoch struct {
	t uint32
	v uint32
}

type vcepoch interface {
	less(vcepoch) bool
	eq(vcepoch) bool
	leq(vcepoch) bool
	sync(vcepoch) vcepoch
	ssync(vcepoch) vcepoch
	add(uint32, uint32) vcepoch
	clone() vcepoch
	set(uint32, uint32) vcepoch
	get(uint32) uint32
	String() string
}

func newEpoch(t uint32, val uint32) epoch {
	return epoch{t, val}
}

func (v epoch) less(v2 vcepoch) bool {
	if v.t == 0 {
		return true
	}
	switch x := v2.(type) {
	case vc2:
		return v.v < x.get(v.t)
	case epoch:
		return v.t == x.t && v.v < x.v
	}

	return false
}
func (v epoch) leq(v2 vcepoch) bool {
	if v.t == 0 {
		return true
	}
	switch x := v2.(type) {
	case vc2:
		return v.v <= x.get(v.t)
	case epoch:
		return v.t == x.t && v.v <= x.v
	}

	return false
}

func (v epoch) eq(v2 vcepoch) bool {
	if v.t == 0 {
		return false
	}
	switch x := v2.(type) {
	case vc2:
		panic("epoch cannot equal vc")
	case epoch:
		return v.t == x.t && v.v == x.v
	}
	return false
}

func (v epoch) sync(v2 vcepoch) vcepoch {
	switch x := v2.(type) {
	case vc2:
		vc := x.clone()

		if v.t != 0 {
			tmp := max(vc.get(v.t), v.v)
			vc.set(v.t, tmp)
		}
		return vc
	case epoch:
		if v.t == x.t {
			v.v = max(v.v, x.v)
			return v
		}
	}
	panic("unknown type!")
	return newvc2()
}

func (v epoch) ssync(v2 vcepoch) vcepoch {
	return v.sync(v2)
}

func (v epoch) add(t uint32, val uint32) vcepoch {
	panic("add on epoch")
}

func (v epoch) clone() vcepoch {
	return newEpoch(v.t, v.v)
}

func (v epoch) set(t uint32, val uint32) vcepoch {
	if v.t == t {
		return newEpoch(t, val)
	}
	v2 := newvc2()
	v2 = v2.internal_set(t, val)
	if v.t != 0 {
		v2 = v2.internal_set(v.t, v.v)
	}
	return v2
}

func (v epoch) get(t uint32) uint32 {
	if v.t == t {
		return v.v
	}
	return 0
}

func (v epoch) String() string {
	return fmt.Sprintf("(T%v@%v)", v.t, v.v)
}

func max(a, b uint32) uint32 {
	if a < b {
		return b
	}
	return a
}

type vc2 []uint32

func (v vc2) String() string {
	s := "["
	for k, v := range v {
		s += fmt.Sprintf("(T%v@%v)", k, v)
	}
	s += "]"
	return s
}

func newvc2() vc2 {
	return make([]uint32, 1)
}

func (v vc2) resize(t uint32) vc2 {
	current := uint32(len(v))
	for current <= t {
		current *= 2
	}

	newV := make([]uint32, current)
	copy(newV, v)

	return newV
}

func (v vc2) set(t uint32, val uint32) vcepoch {
	if t >= uint32(len(v)) {
		v = v.resize(t)
	}
	v[t] = val
	return v
}

func (v vc2) internal_set(t uint32, val uint32) vc2 {
	if t >= uint32(len(v)) {
		v = v.resize(t)
	}
	v[t] = val
	return v
}

func (v vc2) get(t uint32) uint32 {
	if t >= uint32(len(v)) {
		return 0
	}
	return v[t]
}

func (v vc2) less(v2 vcepoch) bool {
	switch x := v2.(type) {
	case vc2:
		if len(v) == 0 {
			return true
		}
		oneRealSmaller := false
		for k := range v {
			idx := uint32(k)
			if v.get(idx) > x.get(idx) {
				return false
			}
			if v.get(idx) < x.get(idx) {
				oneRealSmaller = true
			}
		}
		for k := range x {
			idx := uint32(k)
			if v.get(idx) > x.get(idx) {
				return false
			}
			if v.get(idx) < x.get(idx) {
				oneRealSmaller = true
			}
		}
		return oneRealSmaller
	case epoch:
		return false //vc can never be smaller then a epoch
	}

	return false
}

func (v vc2) eq(v2 vcepoch) bool {
	switch x := v2.(type) {
	case vc2:
		if len(v) != len(x) {
			return false
		}

		for k := range v {
			idx := uint32(k)
			if v.get(idx) != x.get(idx) {
				return false
			}
		}
		for k := range x {
			idx := uint32(k)
			if v.get(idx) != x.get(idx) {
				return false
			}
		}
		return true
	case epoch:
		return false //vc can never be equal to a epoch
	}

	return false
}

func (v vc2) leq(v2 vcepoch) bool {
	switch x := v2.(type) {
	case vc2:
		return v.less(x) || v.eq(x)

	case epoch:
		return false //vc can never be equal to a epoch
	}

	return false
}

func (v vc2) sync(v2 vcepoch) (ret vcepoch) {

	switch x := v2.(type) {
	case vc2:
		for k, vv := range v {
			idx := uint32(k)
			tmp := max(vv, x.get(idx))
			v = v.internal_set(idx, tmp)
			x = x.internal_set(idx, tmp)
		}
		for k, pv := range x {
			idx := uint32(k)
			tmp := max(v.get(idx), pv)
			v = v.internal_set(idx, tmp)
			x = x.internal_set(idx, tmp)
		}
		ret = v
	case epoch:
		panic("sync with epoch not possible!")
	}
	return
}

func (v vc2) ssync(v2 vcepoch) vcepoch {
	switch x := v2.(type) {
	case vc2:
		for k, pv := range x {
			idx := uint32(k)
			vv := v.get(idx)
			tmp := max(vv, pv)
			v = v.internal_set(idx, tmp)
		}
	case epoch:
		panic("sync with epoch not possible!")
	}
	return v
}

func (v vc2) add(t uint32, val uint32) vcepoch {
	if t > uint32(len(v)) {
		v = v.resize(t)
	}
	v = v.internal_set(t, v.get(t)+val)
	return v
}

func (v vc2) expand(t uint32, val uint32) vcepoch {
	v2 := v.clone().(vc2)
	v2 = v2.internal_set(t, val)
	return v2
}

func (v vc2) clone() vcepoch {
	v2 := vc2(make([]uint32, len(v)))
	copy(v2, v)
	return v2
}

// type vc map[uint32]uint32

// func (vc vc) String() string {
// 	s := "["
// 	for k, v := range vc {
// 		s += fmt.Sprintf("(T%v@%v)", k, v)
// 	}
// 	s += "]"
// 	return s
// }

// func newvc() vc {
// 	return make(map[uint32]uint32)
// }

// func (v vc) set(t uint32, val uint32) vcepoch {
// 	v[t] = val
// 	return v
// }
// func (v vc) get(t uint32) uint32 {
// 	return v[t]
// }

// func (v vc) less(v2 vcepoch) bool {
// 	switch x := v2.(type) {
// 	case vc:
// 		if len(v) == 0 {
// 			return true
// 		}
// 		oneRealSmaller := false
// 		for k := range v {
// 			if v[k] > x[k] {
// 				return false
// 			}
// 			if v[k] < x[k] {
// 				oneRealSmaller = true
// 			}
// 		}
// 		for k := range x {
// 			if v[k] > x[k] {
// 				return false
// 			}
// 			if v[k] < x[k] {
// 				oneRealSmaller = true
// 			}
// 		}
// 		return oneRealSmaller
// 	case epoch:
// 		return false //vc can never be smaller then a epoch
// 	}

// 	return false
// }

// func (v vc) eq(v2 vcepoch) bool {
// 	switch x := v2.(type) {
// 	case vc:
// 		if len(v) != len(x) {
// 			return false
// 		}
// 		for k := range v {
// 			if v[k] != x[k] {
// 				return false
// 			}
// 		}
// 		for k := range x {
// 			if v[k] != x[k] {
// 				return false
// 			}
// 		}
// 		return true
// 	case epoch:
// 		return false //vc can never be smaller then a epoch
// 	}

// 	return false
// }

// func (v vc) leq(v2 vcepoch) bool {
// 	switch x := v2.(type) {
// 	case vc:
// 		if len(v) != len(x) {
// 			return false
// 		}
// 		return v.less(x) || v.eq(x)
// 	case epoch:
// 		return false //vc can never be smaller then a epoch
// 	}

// 	return false
// }

// func (v vc) sync(v2 vcepoch) (ret vcepoch) {

// 	switch x := v2.(type) {
// 	case vc:
// 		for k, vv := range v {
// 			pv := x[k]
// 			tmp := max(vv, pv)
// 			v[k] = tmp
// 			x[k] = tmp
// 		}
// 		for k, pv := range x {
// 			vv := v[k]
// 			tmp := max(vv, pv)
// 			v[k] = tmp
// 			x[k] = tmp
// 		}
// 		ret = v
// 	case epoch:
// 		panic("sync with epoch not possible!")
// 	}
// 	return
// }

// func (v vc) ssync(v2 vcepoch) vcepoch {

// 	switch x := v2.(type) {
// 	case vc:
// 		for k, pv := range x {
// 			vv := v[k]
// 			tmp := max(vv, pv)
// 			v[k] = tmp
// 		}
// 	case epoch:
// 		panic("sync with epoch not possible!")
// 	}
// 	return v
// }

// func (v vc) add(t uint32, val uint32) vcepoch {
// 	v[t] = v[t] + val
// 	return v
// }

// func (v vc) expand(t uint32, val uint32) vcepoch {
// 	v2 := v.clone().(vc)
// 	v2[t] = val
// 	return v2
// }

// func (v vc) clone() vcepoch {
// 	v2 := newvc()
// 	for k, v := range v {
// 		v2[k] = v
// 	}
// 	return v2
// }
