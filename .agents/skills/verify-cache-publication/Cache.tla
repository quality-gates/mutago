--------------------------- MODULE Cache ---------------------------
EXTENDS FiniteSets
CONSTANT Values, Positions
VARIABLES cache, first, source, snapshot, expected
vars == <<cache, first, source, snapshot, expected>>
Init == /\ cache = "empty" /\ first = "empty" /\ source = {} /\ snapshot = {} /\ expected = {}
Publish(v) == /\ v \in Values
              /\ cache' = IF cache = "empty" THEN v ELSE cache
              /\ first' = IF first = "empty" THEN v ELSE first
              /\ UNCHANGED <<source, snapshot, expected>>
Clear == /\ cache' = "empty" /\ first' = "empty" /\ UNCHANGED <<source, snapshot, expected>>
Edit(s) == /\ s \subseteq Positions /\ source' = s /\ UNCHANGED <<cache, first, snapshot, expected>>
Store == /\ snapshot' = source /\ expected' = source /\ UNCHANGED <<cache, first, source>>
Next == (\E v \in Values: Publish(v)) \/ Clear \/ (\E s \in SUBSET Positions: Edit(s)) \/ Store
Spec == Init /\ [][Next]_vars
FirstPublisherWins == cache = first
SnapshotStable == snapshot = expected
=============================================================================
