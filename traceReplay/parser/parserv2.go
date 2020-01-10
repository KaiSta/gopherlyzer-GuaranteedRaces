package parser

import (
	"bufio"
	"encoding/binary"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"../util"
)

/*
TODO: types in util.Item were changed to uint32, was uint64 before. Parsing go traces it is necessary to change the memory addresses into counted objects like in
the java tracing. This way uint32 is enough and reduces the memory consumption significantly.

*/

var Parserlock = sync.Mutex{}
var FileNumToString map[uint32]string

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func ParseTracev2(s string) []util.Item {
	// file table first
	FileNumToString = make(map[uint32]string)
	locfilePath := path.Join(s, "locfile.log")
	logfilePath := path.Join(s, "mylog.log")
	//fmt.Println(logfilePath, "\n", locfilePath)

	locfile, err := os.Open(locfilePath)
	check(err)
	defer locfile.Close()
	scanner := bufio.NewScanner(locfile)
	for scanner.Scan() {
		s := scanner.Text()
		splitted := strings.Split(s, ";")
		i, err := strconv.ParseUint(splitted[0], 10, 32)
		check(err)
		Parserlock.Lock()
		FileNumToString[uint32(i)] = splitted[1]
		Parserlock.Unlock()
	}

	logfile, err := os.Open(logfilePath)
	buff := make([]byte, 52)
	events := make([]util.Item, 0, 1000)
	// it := util.Item{}
	// openSelect := false
	for {
		n, err := logfile.Read(buff)
		if n == 0 {
			break //fileend maybe!?
		}
		check(err) //error EOF is handled before with the if statement
		if n != 52 {
			print(n)
			panic("invalid log file line")
		}

		//head := binary.LittleEndian.Uint32(buff)
		// if head&runtime.ISSELECT > 0 {
		// 	if openSelect {
		// 		op := parseOp(buff)
		// 		it.Ops = append(it.Ops, op)
		// 	} else {
		// 		openSelect = true
		// 		it := util.Item{}

		// 		it.Partner = binary.LittleEndian.Uint32(buff[4:])
		// 		it.Thread = binary.LittleEndian.Uint32(buff[8:])
		// 		op := parseOp(buff)
		// 		it.Ops = append(it.Ops, op)
		// 	}
		// } else {
		// 	if openSelect {
		// 		events = append(events, it)
		// 	}
		// 	it := util.Item{}

		// 	it.Partner = binary.LittleEndian.Uint32(buff[4:])
		// 	it.Thread = binary.LittleEndian.Uint32(buff[8:])
		// 	op := parseOp(buff)

		// 	it.Ops = append(it.Ops, op)

		// 	events = append(events, it)
		// }

	}
	return events
}

func parseOp(buff []byte) util.Operation {
	op := util.Operation{}
	op.Ch = uint32(binary.LittleEndian.Uint64(buff[36:]))
	op.BufSize = binary.LittleEndian.Uint32(buff[44:])
	op.BufField = binary.LittleEndian.Uint32(buff[48:])
	op.SourceRef = binary.LittleEndian.Uint32(buff[12:])
	op.Line = binary.LittleEndian.Uint32(buff[16:])
	op.PC = binary.LittleEndian.Uint64(buff[20:])
	op.PPC = binary.LittleEndian.Uint64(buff[28:])

	head := binary.LittleEndian.Uint32(buff)
	if head&1 > 0 {
		op.Kind |= util.POST
	} else {
		op.Kind |= util.PREPARE
	}
	// switch head >> 16 {
	// case runtime.READ:
	// 	op.Kind |= util.READ
	// case runtime.WRITE:
	// 	op.Kind |= util.WRITE
	// case runtime.ATOMICREAD:
	// 	op.Kind |= util.ATOMICREAD
	// case runtime.ATOMICWRITE:
	// 	op.Kind |= util.ATOMICWRITE
	// case runtime.SEND:
	// 	op.Kind |= util.SEND
	// case runtime.RCV:
	// 	op.Kind |= util.RCV
	// case runtime.LOCK:
	// 	op.Kind |= util.SEND
	// 	op.Mutex |= util.LOCK
	// 	op.BufSize = 1
	// case runtime.UNLOCK:
	// 	op.Kind |= util.RCV
	// 	op.Mutex |= util.UNLOCK
	// 	op.BufSize = 1
	// case runtime.SIG:
	// 	op.Kind |= util.SIG
	// 	op.Ch = uint32(op.PPC)
	// case runtime.WAIT:
	// 	op.Kind |= util.WAIT
	// 	op.Ch = uint32(op.PPC)
	// case runtime.CLOSE:
	// 	op.Kind |= util.CLS
	// case runtime.PREBRANCH:
	// 	op.Kind |= util.PREBRANCH
	// case runtime.POSTBRANCH:
	// 	op.Kind |= util.POSTBRANCH
	// }

	return op
}
