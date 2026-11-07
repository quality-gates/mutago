--------------------------- MODULE CustomExec ---------------------------
EXTENDS TLC

CONSTANTS AllowCancel, RestoreNormally

VARIABLES shell, testProcess, contextState, source, backup, engine
vars == <<shell, testProcess, contextState, source, backup, engine>>

\* Start after the bundled script's mv and cp succeeded, when its go test
\* child is running. The real-code probe checks this starting condition.
Init ==
  /\ shell = "testing"
  /\ testProcess = "running"
  /\ contextState = "live"
  /\ source = "mutant"
  /\ backup = "original"
  /\ engine = "waiting"

\* Go's context can expire or be cancelled by the caller.
CancelContext ==
  /\ AllowCancel /\ contextState = "live" /\ engine = "waiting"
  /\ contextState' = "cancelled"
  /\ UNCHANGED <<shell, testProcess, source, backup, engine>>

\* exec.CommandContext's default cancellation kills the script process.
KillScript ==
  /\ contextState = "cancelled"
  /\ shell \in {"testing", "readyToRestore", "restored"}
  /\ shell' = "killed"
  /\ UNCHANGED <<testProcess, contextState, source, backup, engine>>

\* The child can finish before or after the parent is killed or reaped.
FinishTest ==
  /\ testProcess = "running"
  /\ testProcess' = "done"
  /\ shell' = IF shell = "testing" THEN "readyToRestore" ELSE shell
  /\ UNCHANGED <<contextState, source, backup, engine>>

\* clean_up moves source.go.tmp over source.go. RestoreNormally = FALSE is
\* only the deliberate broken variant with the normal clean_up call removed.
RestoreSource ==
  /\ shell = "readyToRestore"
  /\ shell' = "restored"
  /\ source' = IF RestoreNormally THEN "original" ELSE source
  /\ backup' = IF RestoreNormally THEN "none" ELSE backup
  /\ UNCHANGED <<testProcess, contextState, engine>>

ExitScript ==
  /\ shell = "restored"
  /\ shell' = "exited"
  /\ UNCHANGED <<testProcess, contextState, source, backup, engine>>

\* Wait can return after the script exits. Descendants may still be running.
ReturnFromExec ==
  /\ engine = "waiting" /\ shell \in {"exited", "killed"}
  /\ engine' = "returned"
  /\ UNCHANGED <<shell, testProcess, contextState, source, backup>>

Done == /\ engine = "returned" /\ UNCHANGED vars

Next == CancelContext \/ KillScript \/ FinishTest \/ RestoreSource \/
        ExitScript \/ ReturnFromExec \/ Done
Spec == Init /\ [][Next]_vars

\* A caller returning from the custom exec must see its original source.
SourceRestored == engine # "returned" \/ source = "original"
=============================================================================
