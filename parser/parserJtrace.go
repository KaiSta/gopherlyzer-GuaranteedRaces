package parser

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"../util"
)

func ParseJTracev2(s string, n uint64) []util.Item {
	// file table first
	logfilePath := s

	FileNumToString = make(map[uint32]string)
	FileStringToNum := make(map[string]uint32)

	var counter uint32

	// data, err := ioutil.ReadFile(logfilePath)
	// check(err)
	// logfile := string(data)
	// split := strings.Split(logfile, "\n")

	logfile, err := os.Open(logfilePath)
	check(err)
	defer logfile.Close()

	fi, err := logfile.Stat()

	scanner := bufio.NewScanner(logfile)

	events := make([]util.Item, 0, fi.Size()/30)

	var eventCounter int

	for scanner.Scan() && uint64(len(events)) < n {
		s := scanner.Text()
		linesplit := strings.Split(s, ",")

		if len(linesplit) < 5 {
			return events
		}

		it := util.Item{LocalIdx: eventCounter}

		op := util.Operation{}

		tmp, _ := strconv.ParseUint(linesplit[2], 10, 64)
		op.Ch = uint32(tmp)

		op.BufSize = 0
		if linesplit[1] == "LK" || linesplit[1] == "UK" {
			op.BufSize = 1
		}

		stmp := strings.TrimSuffix(linesplit[4], "\r") //windows only

		tmp, err = strconv.ParseUint(stmp, 10, 32)
		//	fmt.Println(tmp, err)
		op.BufField = uint32(tmp)

		split := strings.Split(linesplit[3], ":")

		if p, ok := FileStringToNum[split[0]]; ok {
			op.SourceRef = p
		} else {
			counter++

			if len(split) > 1 {
				FileNumToString[counter] = split[0]
				FileStringToNum[split[0]] = counter

				op.SourceRef = counter
			}
		}
		if len(split) > 1 {
			tmp, _ = strconv.ParseUint(split[1], 10, 32)
			op.Line = uint32(tmp)
		}
		op.PC = 0

		tmp, err = strconv.ParseUint(stmp, 10, 64)
		op.PPC = tmp

		// head := binary.LittleEndian.Uint32(buff)
		// if head&1 > 0 {
		// 	op.Kind |= util.POST
		// } else {
		// 	op.Kind |= util.PREPARE
		// }

		tmp, _ = strconv.ParseUint(linesplit[0], 10, 64)
		it.Thread = uint32(tmp)
		it.Partner = 0

		ops := []util.Operation{}

		switch linesplit[1] {
		case "RD":
			op.Kind |= util.READ
		case "WR":
			op.Kind |= util.WRITE
		case "ARD":
			op.Kind |= util.ATOMICREAD
		case "AWR":
			op.Kind |= util.ATOMICWRITE
		case "LK":
			// it2 := *it.Clone()
			// op2 := op.Clone()
			// op2.Kind = util.SEND | util.PREPARE
			// op2.Mutex = util.LOCK
			// op2.BufSize = 2
			// it2.Ops = append(it2.Ops, op2)
			// events = append(events, it2)
			op.Mutex = util.LOCK
			op.Kind = util.SEND | util.POST
		case "UK":
			// it2 := *it.Clone()
			// op2 := op.Clone()
			// op2.Kind = util.RCV | util.PREPARE
			// op2.Mutex = util.UNLOCK
			// op2.BufSize = 2
			// it2.Ops = append(it2.Ops, op2)
			// events = append(events, it2)
			op.Mutex = util.UNLOCK
			op.Kind = util.RCV | util.POST
		case "SIG":
			op.Kind |= util.SIG
			op.Ch = uint32(op.PPC)
		case "WT":
			op.Kind |= util.WAIT
			op.Ch = uint32(op.PPC)
		case "SIGNALJOIN":
			op.Kind |= util.SIG
			op.Ch = uint32(op.PPC)
		case "WAITJOIN":
			op.Kind |= util.WAIT
			op.Ch = uint32(op.PPC)
		case "SIGNALNOTIFY":
			op.Kind |= util.SIG
			op.Ch = uint32(op.PPC)
		case "WAITNOTIFY":
			op.Kind |= util.WAIT
			op.Ch = uint32(op.PPC)
		}

		ops = append(ops, op)
		it.Ops = ops

		events = append(events, it)

		eventCounter++
	}

	return events
}
