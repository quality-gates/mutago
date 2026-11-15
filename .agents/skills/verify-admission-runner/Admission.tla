--------------------------- MODULE Admission ---------------------------
EXTENDS FiniteSets
CONSTANT Keys
VARIABLES seen, admitted, last
vars == <<seen, admitted, last>>
Init == /\ seen = {} /\ admitted = {} /\ last = "start"
Reject(k) == /\ k \in Keys /\ last' = "reject" /\ UNCHANGED <<seen, admitted>>
Accept(k) == /\ k \in Keys \ seen /\ seen' = seen \cup {k} /\ admitted' = admitted \cup {k} /\ last' = "accept"
Duplicate(k) == /\ k \in seen /\ last' = "duplicate" /\ UNCHANGED <<seen, admitted>>
Next == \E k \in Keys: Reject(k) \/ Accept(k) \/ Duplicate(k)
Spec == Init /\ [][Next]_vars
OnlyAcceptedAreSeen == seen = admitted
TypeOK == /\ seen \subseteq Keys /\ admitted \subseteq Keys /\ last \in {"start", "reject", "accept", "duplicate"}
=============================================================================
