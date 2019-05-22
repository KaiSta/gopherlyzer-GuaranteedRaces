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

 1# |2# | 3#
 ------|-------|------
 wr(x) | ... | ...
 wr(y) | ... | ...
 ... | ... | wr(y)
 ... | rd(y) | ...
 ... | wr(x) | ...