--------------------------- MODULE CoverageBatch ---------------------------
EXTENDS Naturals, FiniteSets
CONSTANT Jobs
VARIABLES pending, failures, received, result
vars == <<pending, failures, received, result>>

Init == /\ pending = Jobs
        /\ failures = {}
        /\ received = {}
        /\ result = "collecting"

ReceiveGood(j) == /\ j \in pending
                  /\ pending' = pending \ {j}
                  /\ received' = received \cup {j}
                  /\ UNCHANGED <<failures, result>>
ReceiveBad(j) == /\ j \in pending
                 /\ pending' = pending \ {j}
                 /\ received' = received \cup {j}
                 /\ failures' = failures \cup {j}
                 /\ UNCHANGED result
Finish == /\ pending = {}
          /\ result = "collecting"
          /\ result' = IF failures = {} THEN "complete" ELSE "error"
          /\ UNCHANGED <<pending, failures, received>>
Done == /\ result \in {"complete", "error"}
        /\ UNCHANGED vars
Next == (\E j \in Jobs: ReceiveGood(j) \/ ReceiveBad(j)) \/ Finish \/ Done
Spec == Init /\ [][Next]_vars
CompleteOnlyWithAllProfiles == result = "complete" => received = Jobs /\ failures = {}
=============================================================================
