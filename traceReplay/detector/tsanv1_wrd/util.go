package tsanwrd

// type vcPair struct {
// 	owner uint32
// 	acq   epoch
// 	rel   vcepoch
// }

type vcPair struct {
	owner uint32
	acq   epoch
	rel   vcepoch
	count int
}

type pair struct {
	*dot
	a bool
}

type intpair struct {
	lk    uint32
	count int
	vc    vcepoch
}

type read struct {
	File uint32
	Line uint32
	T    uint32
}

type dataRace struct {
	raceAcc *dot
	prevAcc *dot
	deps    []intpair
}

type variableHistory struct {
	sourceRef uint32
	t         uint32
	c         uint32
	line      uint16
	isWrite   bool
}
