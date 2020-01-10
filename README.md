# SSHB

## Usage

cd traceReplay
go run main.go -mode sshb -parser javainc -trace ..\benchmark\Test6.log

The result should be:

Variables:1
DynamicRaces:1
UniqueRaces:1
VALID RACES: 1
INVALID RACES: 0
DEF WRD ENOUGH: 0
FALSE POSITIVES: 0
WWRace: 1
WRRace: 0
RWRace: 0
PostProcessing Time: 1.0004ms
Reads:0
Writes:0
Read-Read-Races:0/0
Read-Write-Races:0/0
Write-Read-Races:0/0
Write-Write-Race:1/1
Write-Read-Deps:0
Write-Read-Dep-Races:0/0
ALL:1/1
Missing Edge Cases: 0
Free Counter: 0
Phase1:1/0
Phase2:0/0

It found one happens-before data race (UniqueRaces), which is a guaranteed race (VALID RACES). 