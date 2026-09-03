# Homebrew release operations

Stable tags publish two CGO-free macOS archives to an immutable GitHub release,
then dispatch formula generation to `quality-gates/homebrew-tap`. The GitHub
release is the release commit point: a later tap failure never rolls it back.

## One-time repository setup

1. Enable immutable releases for `quality-gates/mutago`.
2. Create an organization-owned GitHub App with only **Actions: write**, install
   it only on `quality-gates/homebrew-tap`, and keep it off ruleset bypass lists.
3. Create a protected `homebrew` environment in `mutago`. Add
   `HOMEBREW_TAP_APP_ID` and `HOMEBREW_TAP_APP_PRIVATE_KEY` as environment
   secrets, and restrict deployment to stable release tags.
4. In `homebrew-tap`, allow Actions to create pull requests. Protect `main` and
   require the `Test tap` checks. Decide whether passing PRs auto-merge or wait
   for maintainer review; neither choice permits a direct automation push to
   `main`. GitHub requires a maintainer to approve check runs on PRs created by
   the tap's `GITHUB_TOKEN`; the source release workflow waits for those checks
   and reports failure if they do not pass.

## Normal release

Push a stable tag matching `vMAJOR.MINOR.PATCH`. The release workflow validates
the remote tag, runs the complete Go suite and self-mutation quality gates,
builds the Intel and Apple Silicon archives once, and exercises those exact
bytes on macOS 15 and 26 for both architectures. It then publishes and verifies
the immutable release before starting the tap-owned workflow.

The tap workflow treats dispatch inputs as untrusted. It retrieves the release
by ID, verifies the tag and source commit, recalculates both SHA-256 hashes, and
generates the complete formula. It reuses `automation/mutago-vMAJOR.MINOR.PATCH`
and one PR. The PR checks perform four clean installs plus Intel and Apple
Silicon upgrades from any predecessor formula.

## Recovery

Rerun the `Release` workflow with its `tag` input to retry a failed release or
tap stage. Retry behavior is deliberately state-aware:

- A matching draft keeps matching assets and uploads only missing assets.
- A draft with different bytes stops for investigation; automation never
  clobbers an asset.
- A matching immutable release is verified and only the tap dispatch repeats.
- A different immutable release for the same tag is terminal; fix the problem
  under a new version tag.
- Duplicate tap dispatches converge on the same generated formula, branch, and
  PR. A formula already present on `main` is a successful no-op.

If tap publication fails, keep the GitHub release and tag intact. Correct the
tap workflow, branch policy, credentials, or formula check, then redispatch the
same immutable release. Workflow summaries link the source release, target run,
and formula PR for diagnosis.
