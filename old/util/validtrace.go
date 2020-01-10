package util

import "fmt"

func ValidateTrace(items []Item) {
	signalMap := make(map[uint32]struct{})
	lockMap := make(map[uint32]bool)
	wrongLockOrder := 0

	for _, it := range items {
		if it.Ops[0].Kind&LOCK > 0 {
			if x := lockMap[it.Ops[0].Ch]; x {
				wrongLockOrder++
				fmt.Println("Wrong LockOrders:", wrongLockOrder)
			}
			lockMap[it.Ops[0].Ch] = true
		} else if it.Ops[0].Kind&UNLOCK > 0 {
			lockMap[it.Ops[0].Ch] = false
		}
	}

	if wrongLockOrder > 0 {
		panic("invalid trace, wrong lock orders detected!")
	}

	for _, it := range items {
		if it.Ops[0].Kind&SIG > 0 {
			signalMap[it.Ops[0].Ch] = struct{}{}
		}
	}

	for i, it := range items {
		if it.Ops[0].Kind&WAIT > 0 {
			_, ok := signalMap[it.Ops[0].Ch]

			if !ok {
				fmt.Printf("Invalid trace, missing signal for wait. Wait at pos:%v, with id=%v\n", i, it.Ops[0].Ch)
				panic("invalid trace, missing signal for wait")
			}
		}
	}
}
