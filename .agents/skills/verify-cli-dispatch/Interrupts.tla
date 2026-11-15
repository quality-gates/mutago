--------------------------- MODULE Interrupts ---------------------------
VARIABLES registered, first, stopped, cancelled, closed, terminated
vars == <<registered, first, stopped, cancelled, closed, terminated>>
Init == /\ registered = TRUE /\ first = FALSE /\ stopped = FALSE /\ cancelled = FALSE /\ closed = FALSE /\ terminated = FALSE
First == /\ registered /\ ~first /\ first' = TRUE /\ UNCHANGED <<registered, stopped, cancelled, closed, terminated>>
Stop == /\ first /\ registered /\ registered' = FALSE /\ stopped' = TRUE /\ UNCHANGED <<first, cancelled, closed, terminated>>
Cancel == /\ stopped /\ ~cancelled /\ cancelled' = TRUE /\ UNCHANGED <<registered, first, stopped, closed, terminated>>
Second == /\ cancelled /\ ~terminated /\ terminated' = ~registered /\ UNCHANGED <<registered, first, stopped, cancelled, closed>>
Close == /\ ~closed /\ closed' = TRUE /\ UNCHANGED <<registered, first, stopped, cancelled, terminated>>
Done == /\ terminated /\ UNCHANGED vars
Next == First \/ Stop \/ Cancel \/ Second \/ Close \/ Done
Spec == Init /\ [][Next]_vars
UnregisteredBeforeCancellation == cancelled => ~registered
=============================================================================
