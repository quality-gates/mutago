----------------------------- MODULE Walk -----------------------------
EXTENDS Naturals, TLC
CONSTANT N
VARIABLES phase, i, syntax, outcome, mode, cancelled
vars == <<phase, i, syntax, outcome, mode, cancelled>>
Init == /\ phase = "ready" /\ i = 1 /\ syntax = "original"
        /\ outcome = "none" /\ mode \in {"sync", "channel"} /\ cancelled = FALSE
Apply == /\ phase = "ready" /\ i <= N /\ ~cancelled
         /\ phase' = "applied" /\ syntax' = "changed"
         /\ UNCHANGED <<i, outcome, mode, cancelled>>
Callback == /\ phase = "applied"
            /\ outcome' \in (IF mode = "sync" THEN {"ok", "error", "cancel", "panic"} ELSE {"ok"})
            /\ cancelled' = (outcome' = "cancel")
            /\ phase' = "unwinding"
            /\ UNCHANGED <<i, syntax, mode>>
Reset == /\ phase = "unwinding" /\ syntax' = "original"
         /\ phase' = IF outcome = "error" THEN "failed"
                     ELSE IF outcome = "panic" THEN "panicked"
                     ELSE IF mode = "channel" THEN "resetNotice" ELSE "advance"
         /\ UNCHANGED <<i, outcome, mode, cancelled>>
AckReset == /\ phase = "resetNotice" /\ phase' = "advance"
            /\ UNCHANGED <<i, syntax, outcome, mode, cancelled>>
Advance == /\ phase = "advance"
           /\ phase' = IF cancelled THEN "cancelled" ELSE IF i = N THEN "done" ELSE "ready"
           /\ i' = IF cancelled \/ i = N THEN i ELSE i + 1
           /\ UNCHANGED <<syntax, outcome, mode, cancelled>>
Done == /\ phase \in {"failed", "panicked", "cancelled", "done"} /\ UNCHANGED vars
Next == Apply \/ Callback \/ Reset \/ AckReset \/ Advance \/ Done
Spec == Init /\ [][Next]_vars
OriginalOutsideMutation == phase \in {"ready", "resetNotice", "advance", "failed", "panicked", "cancelled", "done"} => syntax = "original"
TypeOK == /\ phase \in {"ready", "applied", "unwinding", "resetNotice", "advance", "failed", "panicked", "cancelled", "done"}
          /\ i \in 1..N /\ syntax \in {"original", "changed"}
          /\ mode \in {"sync", "channel"} /\ cancelled \in BOOLEAN
=============================================================================
