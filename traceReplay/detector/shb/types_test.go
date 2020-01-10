package shb

import (
	"testing"
)

func TestResize(t *testing.T) {
	v := newvc2()

	v = v.resize(4)

	if len(v) < 8 {
		t.Errorf("Size of v is %v, expected %v!", len(v), 8)
	}
}

func TestResizeContent(t *testing.T) {
	v := vc2([]uint32{1, 2, 3, 4, 5})

	v = v.resize(6)

	if len(v) < 10 {
		t.Errorf("Size of v is %v, expected %v!", len(v), 10)
	}

	for i := 0; i < 5; i++ {
		if v[i] != uint32(i+1) {
			t.Errorf("Wrong values after resize. Expected %v, got %v @ i=%v.", vc2([]uint32{1, 2, 3, 4, 5}), v, i)
		}
	}
}

func TestSetExistingPos(t *testing.T) {
	v1 := vc2([]uint32{1, 2, 3, 4, 5})

	v1 = v1.set(0, 5).(vc2)
	v1 = v1.set(1, 4).(vc2)
	v1 = v1.set(2, 3).(vc2)
	v1 = v1.set(3, 2).(vc2)
	v1 = v1.set(4, 1).(vc2)

	for i, v := range []uint32{5, 4, 3, 2, 1} {
		if v != v1[i] {
			t.Errorf("Set with existing positions failed. Expected %v, got %v.", v, v1[i])
		}
	}
}

func TestSetGreaterPos(t *testing.T) {
	v1 := vc2([]uint32{0, 1, 2, 3, 4})

	v1 = v1.set(8, 8).(vc2)

	if len(v1) != 10 {
		t.Errorf("Resize for set greater pos failed. Expected %v, got %v.", 10, len(v1))
	}

	for i, v := range []uint32{0, 1, 2, 3, 4, 0, 0, 0, 8, 0} {
		if v != v1[i] {
			t.Errorf("Set with existing positions failed. Expected %v, got %v.", v, v1[i])
		}
	}
}

func TestSSyncEqualSize(t *testing.T) {
	v1 := vc2([]uint32{1, 2, 3, 4, 5})
	v2 := vc2([]uint32{5, 4, 3, 2, 1})

	v1.ssync(v2)

	for i, v := range []uint32{5, 4, 3, 4, 5} {
		if v != v1[i] {
			t.Errorf("Sync with equal size failed. Expected %v, got %v.", v, v1[i])
		}
	}
}

func TestSSyncDifferentSize1(t *testing.T) {
	v1 := vc2([]uint32{1, 2, 3, 4, 5})
	v2 := vc2([]uint32{5, 4})

	v1.ssync(v2)

	if len(v1) != 5 {
		t.Errorf("Sync with different size failed. Expected size %v, got %v.", 5, len(v1))
	}

	for i, v := range []uint32{5, 4, 3, 4, 5} {
		if v != v1[i] {
			t.Errorf("Sync with different size failed. Expected %v, got %v.", v, v1[i])
		}
	}
}

func TestSSyncDifferentSize2(t *testing.T) {
	v1 := vc2([]uint32{1, 2, 3, 4, 5})
	v2 := vc2([]uint32{5, 4})

	v2 = v2.ssync(v1).(vc2)

	if len(v2) != 8 {
		t.Errorf("Sync with different size failed. Expected size %v, got %v.", 5, len(v2))
	}

	for i, v := range []uint32{5, 4, 3, 4, 5} {
		if v != v2[i] {
			t.Errorf("Sync with different size failed. Expected %v, got %v.", v, v1[i])
		}
	}
}

func TestLess1(t *testing.T) {
	v1 := vc2([]uint32{1, 2, 3, 4, 5})
	v2 := vc2([]uint32{1, 2, 3, 3, 5})

	if !v2.less(v1) {
		t.Errorf("Less between two vc were one is smaller failed. VC1 %v, VC2 %v.", v1, v2)
	}
	if v1.less(v2) {
		t.Errorf("Less between two vc were one is smaller failed. VC1 %v, VC2 %v.", v1, v2)

	}
}

func TestLess2(t *testing.T) {
	v1 := vc2([]uint32{1, 2, 3, 4, 5})
	v2 := vc2([]uint32{1, 2, 3, 4, 5})

	if v2.less(v1) {
		t.Errorf("Less between two identical vcs failed. VC1 %v, VC2 %v.", v1, v2)
	}
}

func TestLEQ1(t *testing.T) {
	v1 := vc2([]uint32{1, 2, 3, 4, 5})
	v2 := vc2([]uint32{1, 2, 3, 3, 5})

	if !v2.leq(v1) {
		t.Errorf("Leq between two vc were one is smaller failed. VC1 %v, VC2 %v.", v1, v2)
	}
	if v1.leq(v2) {
		t.Errorf("Leq between two vc were one is smaller failed. VC1 %v, VC2 %v.", v1, v2)

	}
}

func TestLEQ2(t *testing.T) {
	v1 := vc2([]uint32{1, 2, 3, 4, 5})
	v2 := vc2([]uint32{1, 2, 3, 4, 5})

	if !v2.leq(v1) {
		t.Errorf("Leq between two identical vcs failed. VC1 %v, VC2 %v.", v1, v2)
	}
}

func TestLEQ3(t *testing.T) {
	v1 := vc2([]uint32{0, 1})
	v2 := vc2([]uint32{0, 2, 1, 0})

	if !v1.leq(v2) {
		t.Errorf("Leq between %v and %v failed.", v1, v2)
	}
}
