---
name: verify-cache-publication
description: Verify cache first-publisher behavior and annotation snapshot ownership when changing package caching or block exclusions.
---

# Verify cache and annotation publication

**Properties:** Competing lazy package loads return the first entry published for a directory until the cache is cleared or bulk-prepared. Each processor publishes a copy of its own statement exclusions, keyed by syntax-tree identity. Recollection replaces that owner's entries, and closing an owner removes only its entries.

**Harm:** Replacing the first loaded entry can cause concurrent callers to use different syntax/type information for one package. Cross-file position collisions can suppress unrelated mutations; removing another owner's entries can expose mutations that annotations exclude. Unreleased owners retain syntax trees.

## Coverage and sync

Last matched to `e763cd8`: `packageCache.publish`, `packageCache.prepare`, `packageCache.clear`, `packageCache.packageFor` in `internal/parser/parse.go`, and `Processor.collectNodesForBlockStmt`, `Processor.Close`, `HandleBlockStmt` in `internal/annotation/block.go`. Compare changes to these operations with the model and probes, including uncommitted changes. Advance the sync commit only after confirming that verification describes the code.

TLC checks atomic package publish and clear alongside independent edits, stores and releases by two owners over two statement identities. The registry invariant checks that every statement retains exactly its owning processors; snapshot stability checks that later source edits do not change published exclusions. The Go cache probe exhausts five-operation sequences of two publishes, clear, and bulk preparation. The annotation probes cover line and regex combinations, later collector changes, identical positions on distinct statements, shared statement ownership and cleanup. Scratch overlays break the first-publisher guard, statement identity, and owner cleanup; each must fail for the stated reason, and restored code must pass. The identity break is also exercised through the real statement mutator.

The model abstracts critical sections as atomic. It does not model `prepare`, package loading errors, other indexes, race scheduling, or simultaneous use of a processor's mutable collector maps. Processor owners must call Close when mutation finishes; the engine defers it on all exits. Multiple owners of the same AST statement contribute exclusions until individually released. The model does not prove the Go mutex correct or claim liveness. An in-flight load may publish after clear in this model; verify generation handling separately when present.

## Run and interpret

Run `./.agents/skills/verify-cache-publication/check.sh` from the repository root. Go, Java, Python 3 and Bash are required; the first run downloads the SHA-256-pinned TLC 1.7.4 jar. Set `MUTAGO_TLC_JAR` to use an existing copy. The check does not edit the checkout.

Read every model result with `/statechart`, using `Cache.tla`, `Cache.cfg` and `view.mjs`. Replay a counterexample against the named operations. A setup failure or a probe failure unrelated to the broken property is not a successful sensitivity check. If a scratch mutation no longer applies, inspect the code and construct a real break of the same property, then restore it and rerun before advancing the sync point.
