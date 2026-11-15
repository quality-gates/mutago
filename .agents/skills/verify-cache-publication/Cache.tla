--------------------------- MODULE Cache ---------------------------
EXTENDS FiniteSets
CONSTANT Values, Owners, Statements
VARIABLES cache, first, source, snapshot, expected, registry
vars == <<cache, first, source, snapshot, expected, registry>>
Init == /\ cache = "empty" /\ first = "empty"
        /\ source = [o \in Owners |-> {}] /\ snapshot = [o \in Owners |-> {}]
        /\ expected = [o \in Owners |-> {}] /\ registry = [s \in Statements |-> {}]
Publish(v) == /\ v \in Values
              /\ cache' = IF cache = "empty" THEN v ELSE cache
              /\ first' = IF first = "empty" THEN v ELSE first
              /\ UNCHANGED <<source, snapshot, expected, registry>>
Clear == /\ cache' = "empty" /\ first' = "empty" /\ UNCHANGED <<source, snapshot, expected, registry>>
Edit(o, s) == /\ source' = [source EXCEPT ![o] = s]
              /\ UNCHANGED <<cache, first, snapshot, expected, registry>>
Store(o) == /\ snapshot' = [snapshot EXCEPT ![o] = source[o]]
            /\ expected' = [expected EXCEPT ![o] = source[o]]
            /\ registry' = [s \in Statements |-> (registry[s] \ {o}) \cup (IF s \in source[o] THEN {o} ELSE {})]
            /\ UNCHANGED <<cache, first, source>>
Release(o) == /\ snapshot' = [snapshot EXCEPT ![o] = {}]
              /\ expected' = [expected EXCEPT ![o] = {}]
              /\ registry' = [s \in Statements |-> registry[s] \ {o}]
              /\ UNCHANGED <<cache, first, source>>
Next == (\E v \in Values: Publish(v)) \/ Clear \/ (\E o \in Owners, s \in SUBSET Statements: Edit(o,s)) \/ (\E o \in Owners: Store(o) \/ Release(o))
Spec == Init /\ [][Next]_vars
FirstPublisherWins == cache = first
SnapshotStable == snapshot = expected
OwnerIsolation == \A o \in Owners, s \in Statements: (o \in registry[s]) = (s \in snapshot[o])
=============================================================================
