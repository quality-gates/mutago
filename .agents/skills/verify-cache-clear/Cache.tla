--------------------------- MODULE Cache ---------------------------
CONSTANT Values
VARIABLES cache, first, loading, fresh
vars == <<cache, first, loading, fresh>>
Init == /\ cache = "empty" /\ first = "empty" /\ loading = FALSE /\ fresh = FALSE
StartLoad == /\ ~loading /\ loading' = TRUE /\ fresh' = TRUE /\ UNCHANGED <<cache, first>>
Publish(v) == /\ loading /\ v \in Values
              /\ cache' = IF fresh /\ cache = "empty" THEN v ELSE cache
              /\ first' = IF fresh /\ first = "empty" THEN v ELSE first
              /\ loading' = FALSE /\ UNCHANGED <<fresh>>
Clear == /\ cache' = "empty" /\ first' = "empty" /\ fresh' = FALSE /\ UNCHANGED <<loading>>
Next == StartLoad \/ (\E v \in Values: Publish(v)) \/ Clear
Spec == Init /\ [][Next]_vars
FirstPublisherWins == cache = first
NoStalePublication == ~fresh => cache = "empty"
=============================================================================
