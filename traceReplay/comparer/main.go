package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

type race struct {
	loc1 string
	loc2 string
}

func main() {
	file1 := flag.String("file1", "", "file1")
	file2 := flag.String("file2", "", "file2")
	single := flag.Bool("single", false, "for wcp/shb")

	flag.Parse()

	logfile, _ := os.Open(*file1)
	logfile2, _ := os.Open(*file2)

	defer logfile.Close()
	defer logfile2.Close()

	races1 := parse(logfile)
	races2 := parse(logfile2)

	notinRaces1 := 0
	onlyinRaces1 := len(races1)

	if !*single {

		for _, r := range races2 {
			notin := 0
			for _, r2 := range races1 {
				if r.loc1 == r2.loc1 && r.loc2 == r2.loc2 {
					// race is in both sets
					onlyinRaces1--
				} else {
					notin++
				}
			}
			if notin == len(races1) {
				notinRaces1++
			}
		}
		fmt.Println("Races1:", len(races1))
		fmt.Println("Races2:", len(races2))
		fmt.Println("Alt Scheds:", notinRaces1)
		fmt.Println("Missed:", onlyinRaces1)
	} else {
		locs1 := make(map[string]struct{})
		locs2 := make(map[string]struct{})

		notinRaces1 = 0
		onlyinRaces1 = 0

		for _, r := range races1 {
			locs1[r.loc1] = struct{}{}
			locs1[r.loc2] = struct{}{}
		}
		for _, r := range races2 {
			locs2[r.loc2] = struct{}{}
		}

		for k := range locs2 {
			if _, ok := locs1[k]; !ok {
				notinRaces1++
			}
		}
		for k := range locs1 {
			if _, ok := locs2[k]; !ok {
				onlyinRaces1++
			}
		}

		fmt.Println("locations1:", len(locs1))
		fmt.Println("locations2:", len(locs2))
	}
	fmt.Println("Races1:", len(races1))
	fmt.Println("Races2:", len(races2))
	fmt.Println("Alt Scheds:", notinRaces1)
	fmt.Println("Missed:", onlyinRaces1)
}

func parse(file *os.File) []race {
	scanner := bufio.NewScanner(file)
	races := make([]race, 0)

	for scanner.Scan() {
		s := scanner.Text()
		linesplit := strings.Split(s, ",")

		if len(linesplit) < 2 {
			return races
		}

		races = append(races, race{linesplit[0], linesplit[1]})
	}
	return races
}
