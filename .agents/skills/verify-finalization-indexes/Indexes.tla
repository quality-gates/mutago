----------------------------- MODULE Indexes -----------------------------
EXTENDS FiniteSets
CONSTANT Nodes, Paths
VARIABLES seen, covered, safetyBuilt, epoch, safetyEpoch, cached, values
vars == <<seen, covered, safetyBuilt, epoch, safetyEpoch, cached, values>>
Source(p) == IF p = "hit" THEN "found" ELSE "missing"
Init == /\ seen = {} /\ covered = {} /\ safetyBuilt = FALSE /\ epoch = FALSE /\ safetyEpoch = FALSE
        /\ cached = {} /\ values = [p \in Paths |-> "unset"]
Ensure(r) == /\ r \in SUBSET Nodes
             /\ IF \E old \in covered: r \subseteq old
                   THEN /\ UNCHANGED <<seen, covered>>
                   ELSE /\ seen' = seen \cup r /\ covered' = covered \cup {r}
             /\ UNCHANGED <<safetyBuilt, epoch, safetyEpoch, cached, values>>
BuildSafety == /\ ~safetyBuilt /\ safetyBuilt' = TRUE /\ safetyEpoch' = epoch
               /\ UNCHANGED <<seen, covered, epoch, cached, values>>
ClearSafety == /\ safetyBuilt' = FALSE /\ UNCHANGED <<seen, covered, epoch, safetyEpoch, cached, values>>
ClearIndexes == /\ seen' = {} /\ covered' = {} /\ safetyBuilt' = FALSE /\ epoch' = ~epoch
                /\ UNCHANGED <<safetyEpoch, cached, values>>
Resolve(p) == /\ p \in Paths
              /\ IF p \in cached
                    THEN UNCHANGED <<cached, values>>
                    ELSE /\ cached' = cached \cup {p} /\ values' = [values EXCEPT ![p] = Source(p)]
              /\ UNCHANGED <<seen, covered, safetyBuilt, epoch, safetyEpoch>>
Next == (\E r \in SUBSET Nodes: Ensure(r)) \/ BuildSafety \/ ClearSafety \/ ClearIndexes \/ (\E p \in Paths: Resolve(p))
Spec == Init /\ [][Next]_vars
CoveredNodesCollected == \A r \in covered: r \subseteq seen
CachedResolutionCorrect == \A p \in cached: values[p] = Source(p)
SafetyMatchesEpoch == safetyBuilt => safetyEpoch = epoch
TypeOK == /\ seen \subseteq Nodes /\ covered \subseteq SUBSET Nodes /\ safetyBuilt \in BOOLEAN
          /\ epoch \in BOOLEAN /\ safetyEpoch \in BOOLEAN
          /\ cached \subseteq Paths /\ values \in [Paths -> {"unset", "found", "missing"}]
=============================================================================
