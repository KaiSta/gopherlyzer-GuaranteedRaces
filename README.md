# gopherlyzer-GuaranteedRaces

Prototype implementation of our analysis described in *Data Race Prediction for Inaccurate Traces*.

## Description

  Happens-before based data race prediction methods infer from a trace of events a partial order
  to check if one event happens before another event.
  If two two write events are unordered, they are in a race.
  We observe that common tracing methods provide no guarantee that the
  trace order corresponds to an actual program run.
  The consequence  of inaccurate tracing is that results (races) reported are inaccurate.
  We introduce diagnostic methods to examine
  if (1) a race is guaranteed to be correct regardless of the
  any potential inaccuracies, (2) maybe is incorrect due to inaccurate tracing.
  We have fully implemented the approach and provide for an empirical comparison
  with state of the art happens-before based race predictors such as FastTrack and SHB.

  ## How to use

We use a small example program to show the process and the results of our prototype. In tests/simple.log a complete trace that resembles the following sequence of event is given.

 1# |2# | 3#
 ------|-------|------
 wr(x) | ... | ...
 wr(y) | ... | ...
 ... | ... | wr(y)
 ... | rd(y) | ...
 ... | wr(x) | ...

 
 We find data race between the writes on *y* and the read on *y* with both writes. To test the trace with the algorithm presented in our paper, we use the command:

 ```
go run main.go -mode sshb -parser java -trace tests/simple.log
 ```

The result contains shows the guaranteed races as pairs of code locations.

 ```
...
DynamicRaces:4
UniqueRaces:4
1/4
WWRACE:Test.java:2, Test.java:3
ALL:1/1
2/4
WRRACE:Test.java:2, Test.java:4
ALL:2/2
3/4
WRRACE:Test.java:3, Test.java:4
ALL:3/3
4/4
Max writes:2
Avg writes:2
PostProcessing Time: 0s
Reads:0
Writes:0
Read-Read-Races:0/0
Read-Write-Races:0/0
Write-Read-Races:2/2
Write-Write-Race:1/1
Write-Read-Deps:0
Write-Read-Dep-Races:0/0
ALL:3/3

...
 ```