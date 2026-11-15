---
name: verify-cache-publication
description: Verify cache first-publisher behavior and annotation snapshot ownership when changing package caching or block exclusions.
---

# Verify cache and annotation publication

**Properties:** Competing lazy package loads return the first entry published for a directory until the cache is cleared or bulk-prepared. Published block exclusions remain unchanged when the collector's maps are subsequently changed. A later collection replaces the previous snapshot.

**Harm:** Replacing the first loaded entry can cause concurrent callers to use different syntax/type information for one package. Sharing collector maps lets later edits silently alter which mutations are excluded.

## Coverage and sync

Last matched to `b6995b8`: `packageCache.publish`, `packageCache.prepare`, `packageCache.clear`, `packageCache.packageFor` in `internal/parser/parse.go`, and `Processor.collectNodesForBlockStmt`, `copyBlockExclusions`, `HandleBlockStmt` in `internal/annotation/annotation.go` and `block.go`. Compare every change to those operations with the model and probes, including uncommitted changes. Update the verification for intended behavior, and move the sync commit forward only after confirming the verification still describes the real code.

TLC checks all orderings of atomic publish, clear, collector edit, and snapshot store for two candidate package values and two positions. `Publish` represents the locked double check and assignment; `Clear` represents its locked reset. `Store` represents the atomic pointer swap after building a copy; `Edit` changes collector-owned data. The real Go cache probe exhausts five-operation sequences of two publishes, clear, and bulk preparation, and checks returned and stored package entries. The real annotation probe covers all combinations of line and regex exclusions, collector-map changes after publication, and a subsequent empty snapshot. Scratch overlays break the first-publisher guard and map copying; each broken variant must fail for the stated property and the restored code must pass.

The model abstracts each critical section and pointer swap as atomic; it does not model `prepare`, file index contents, package loading errors, `types.Info` AST indexes, safety indexes, coverage path caches, mutator selector internals, race scheduling or multiple independent mutation runs. It intentionally permits an in-flight load to publish after a clear, matching the code. Snapshot publication is global: independent concurrent collections can replace one another. No run isolation or liveness claim is made. The probe does not prove Go's mutex or atomic implementation correct.

## Run and interpret

Run `./.agents/skills/verify-cache-publication/check.sh` from the repository root. Go, Java, Python 3 and Bash are required; the first run downloads the SHA-256-pinned TLC 1.7.4 jar. Set `MUTAGO_TLC_JAR` to use an existing copy. The check does not edit the checkout.

Read every model result with `/statechart`, using `Cache.tla`, `Cache.cfg` and `view.mjs`. Replay a counterexample against the named operations. A setup failure or a probe failure unrelated to the broken property is not a successful sensitivity check. If the scratch mutation no longer applies, inspect the code and construct a real break of the same property, then restore it and rerun before advancing the sync point.
