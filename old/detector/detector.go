package race

import "../util"

func createThreads(items []util.Item) map[uint32]util.Thread {
	tmap := make(map[uint32]util.Thread)
	for i, item := range items {
		t, ok := tmap[item.Thread]
		if !ok {
			t.ID = item.Thread
		}
		// if len(t.Events) > 0 { //check if its the first event
		// 	//inter thread hb link, last event can reach current event
		// 	//t.Events[len(t.Events)-1].Next = append(t.Events[len(t.Events)-1].Next, &items[i])
		// 	t.Events[len(t.Events)-1].InternalNext = &items[i]
		// 	items[i].Prev = t.Events[len(t.Events)-1]
		// }
		items[i].LocalIdx = len(t.Events)
		t.Events = append(t.Events, &items[i])

		tmap[item.Thread] = t
	}
	return tmap
}

func createMasterThread(items []util.Item) map[uint64]util.Thread {
	tmap := make(map[uint64]util.Thread)
	for i := range items {
		t := tmap[1]
		t.Events = append(t.Events, &items[i])
		tmap[1] = t
	}
	return tmap
}

func getAsyncChans2(items []util.Item) map[uint32]util.AsyncChan {
	chans := make(map[uint32]util.AsyncChan)
	for _, i := range items {
		for _, o := range i.Ops {
			if o.BufSize == 0 {
				continue
			}
			//extend here to add it as special channel for rw locks
			if o.Mutex == 0 {
				if _, ok := chans[o.Ch]; !ok {
					achan := util.AsyncChan{BufSize: o.BufSize, Buf: make([]util.BufField, o.BufSize)}
					for i := range achan.Buf {
						achan.Buf[i].VC = util.NewVC()
					}
					chans[o.Ch] = achan
				}
			} else {
				if _, ok := chans[o.Ch]; !ok {
					achan := util.AsyncChan{BufSize: 2, Buf: make([]util.BufField, 2), IsLock: true}
					for i := range achan.Buf {
						achan.Buf[i].VC = util.NewVC()
					}
					if o.Mutex == util.RLOCK {
						achan.IsLock = true
					}
					chans[o.Ch] = achan
				} else if o.Mutex == util.RLOCK {
					c := chans[o.Ch]
					c.IsLock = true
					chans[o.Ch] = c
				}
			}
		}
	}
	return chans
}
