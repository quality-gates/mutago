--------------------------- MODULE CustomExec ---------------------------
EXTENDS TLC

CONSTANTS AllowCancel, ReplaceSource

VARIABLES shell, testProcess, contextState, source, overlay, engine
vars == <<shell, testProcess, contextState, source, overlay, engine>>

Init ==
  /\ shell = "preparing"
  /\ testProcess = "absent"
  /\ contextState = "live"
  /\ source = "original"
  /\ overlay = "absent"
  /\ engine = "waiting"

StartTest ==
  /\ shell = "preparing"
  /\ shell' = "testing"
  /\ testProcess' = "running"
  /\ overlay' = "mutant"
  /\ source' = IF ReplaceSource THEN "mutant" ELSE source
  /\ UNCHANGED <<contextState, engine>>

CancelContext ==
  /\ AllowCancel /\ contextState = "live" /\ engine = "waiting"
  /\ contextState' = "cancelled"
  /\ UNCHANGED <<shell, testProcess, source, overlay, engine>>

\* exec.CommandContext kills the script; its test child can outlive it.
KillScript ==
  /\ contextState = "cancelled"
  /\ shell \in {"preparing", "testing", "testDone"}
  /\ shell' = "killed"
  /\ UNCHANGED <<testProcess, contextState, source, overlay, engine>>

FinishTest ==
  /\ testProcess = "running"
  /\ testProcess' = "done"
  /\ shell' = IF shell = "testing" THEN "testDone" ELSE shell
  /\ UNCHANGED <<contextState, source, overlay, engine>>

ExitScript ==
  /\ shell = "testDone"
  /\ shell' = "exited"
  /\ UNCHANGED <<testProcess, contextState, source, overlay, engine>>

\* Wait may return while a descendant is still running.
ReturnFromExec ==
  /\ engine = "waiting" /\ shell \in {"exited", "killed"}
  /\ engine' = "returned"
  /\ UNCHANGED <<shell, testProcess, contextState, source, overlay>>

Done == /\ engine = "returned" /\ UNCHANGED vars

Next == StartTest \/ CancelContext \/ KillScript \/ FinishTest \/
        ExitScript \/ ReturnFromExec \/ Done
Spec == Init /\ [][Next]_vars

SourcePreserved == source = "original"
=============================================================================
