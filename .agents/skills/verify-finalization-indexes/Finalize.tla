----------------------------- MODULE Finalize -----------------------------
EXTENDS Naturals
VARIABLES id, found, update, errors, reportFails, stage, outcome
vars == <<id, found, update, errors, reportFails, stage, outcome>>
Init == /\ id \in BOOLEAN /\ found \in BOOLEAN /\ update \in BOOLEAN /\ errors \in BOOLEAN /\ reportFails \in BOOLEAN /\ stage = "start" /\ outcome = "none"
Select == /\ stage = "start"
          /\ stage' = IF id /\ ~found THEN "missing" ELSE IF update /\ ~errors THEN "baseline" ELSE "report"
          /\ UNCHANGED <<id, found, update, errors, reportFails, outcome>>
Finish == /\ stage \in {"missing", "baseline", "report"}
          /\ outcome' = IF stage = "missing" THEN "error" ELSE IF stage = "baseline" THEN "updated" ELSE IF reportFails \/ errors THEN "error" ELSE IF id THEN "individual" ELSE "gates"
          /\ stage' = "done"
          /\ UNCHANGED <<id, found, update, errors, reportFails>>
Done == /\ stage = "done" /\ UNCHANGED vars
Next == Select \/ Finish \/ Done
Spec == Init /\ [][Next]_vars
NoUpdateOnErrors == outcome = "updated" => ~errors /\ update /\ (~id \/ found)
MissingWins == (stage = "done" /\ id /\ ~found) => outcome = "error"
ReportsPrecedeGates == (stage = "done" /\ stage # "baseline" /\ outcome \in {"individual", "gates"}) => ~reportFails /\ ~errors
=============================================================================
