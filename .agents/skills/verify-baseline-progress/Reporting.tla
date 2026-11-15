--------------------------- MODULE Reporting ---------------------------
EXTENDS FiniteSets
CONSTANT Ids
VARIABLES present, known, escaped, new, line
vars == <<present, known, escaped, new, line>>
Init == /\ present = FALSE /\ known = {} /\ escaped = {} /\ new = {} /\ line = "clear"
Capture(s) == /\ s \subseteq Ids /\ present' = TRUE /\ known' = s /\ new' = escaped \ s /\ UNCHANGED <<escaped, line>>
Missing == /\ present' = FALSE /\ known' = {} /\ new' = escaped /\ UNCHANGED <<escaped, line>>
Compare(s) == /\ s \subseteq Ids /\ escaped' = s /\ new' = (IF present THEN s \ known ELSE s) /\ UNCHANGED <<present, known, line>>
Draw == /\ line' = "drawn" /\ UNCHANGED <<present, known, escaped, new>>
Clear == /\ line' = "clear" /\ UNCHANGED <<present, known, escaped, new>>
Next == (\E s \in SUBSET Ids: Capture(s) \/ Compare(s)) \/ Missing \/ Draw \/ Clear
Spec == Init /\ [][Next]_vars
CorrectEscapes == new = IF present THEN escaped \ known ELSE escaped
TypeOK == /\ present \in BOOLEAN /\ known \subseteq Ids /\ escaped \subseteq Ids /\ new \subseteq Ids /\ line \in {"clear", "drawn"}
=============================================================================
