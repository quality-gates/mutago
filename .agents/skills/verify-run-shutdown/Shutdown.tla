--------------------------- MODULE Shutdown ---------------------------
EXTENDS FiniteSets, Naturals
CONSTANT Jobs
VARIABLES queued, active, completed, skipped, closing, workersDone, progressDone, summaries, removed
vars == <<queued, active, completed, skipped, closing, workersDone, progressDone, summaries, removed>>
Init == /\ queued = Jobs /\ active = {} /\ completed = {} /\ skipped = {} /\ closing = FALSE /\ workersDone = FALSE /\ progressDone = FALSE /\ summaries = 0 /\ removed = FALSE
Start(j) == /\ j \in queued /\ queued' = queued \ {j} /\ active' = active \cup {j} /\ UNCHANGED <<completed, skipped, closing, workersDone, progressDone, summaries, removed>>
Skip(j) == /\ j \in queued /\ queued' = queued \ {j} /\ skipped' = skipped \cup {j} /\ UNCHANGED <<active, completed, closing, workersDone, progressDone, summaries, removed>>
Complete(j) == /\ j \in active /\ active' = active \ {j} /\ completed' = completed \cup {j} /\ UNCHANGED <<queued, skipped, closing, workersDone, progressDone, summaries, removed>>
Close == /\ ~closing /\ closing' = TRUE /\ UNCHANGED <<queued, active, completed, skipped, workersDone, progressDone, summaries, removed>>
WaitWorkers == /\ closing /\ queued = {} /\ active = {} /\ ~workersDone /\ workersDone' = TRUE /\ UNCHANGED <<queued, active, completed, skipped, closing, progressDone, summaries, removed>>
StopProgress == /\ workersDone /\ ~progressDone /\ progressDone' = TRUE /\ UNCHANGED <<queued, active, completed, skipped, closing, workersDone, summaries, removed>>
Summarize == /\ progressDone /\ summaries = 0 /\ summaries' = 1 /\ UNCHANGED <<queued, active, completed, skipped, closing, workersDone, progressDone, removed>>
Remove == /\ summaries = 1 /\ ~removed /\ removed' = TRUE /\ UNCHANGED <<queued, active, completed, skipped, closing, workersDone, progressDone, summaries>>
Done == /\ removed /\ UNCHANGED vars
Next == (\E j \in Jobs: Start(j) \/ Skip(j) \/ Complete(j)) \/ Close \/ WaitWorkers \/ StopProgress \/ Summarize \/ Remove \/ Done
Spec == Init /\ [][Next]_vars
SafeSummary == summaries <= 1 /\ (summaries = 1 => workersDone /\ progressDone /\ active = {} /\ queued = {})
SafeRemoval == removed => summaries = 1
=============================================================================
