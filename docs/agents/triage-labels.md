# Triage Labels

The skills speak in terms of two canonical category roles and five canonical state roles. This file maps those roles to the actual label strings used in `quality-gates/mutago`'s GitHub issue tracker.

## Category labels

| Canonical role | Label in this repo | Meaning                          |
| --------------- | ------------------- | --------------------------------- |
| `bug`           | `bug`               | Something isn't working           |
| `enhancement`   | `enhancement`       | New feature or improvement        |

## State labels

| Canonical role   | Label in this repo | Meaning                                  |
| ----------------- | ------------------- | ----------------------------------------- |
| `needs-triage`    | `needs-triage`      | Maintainer needs to evaluate this issue  |
| `needs-info`      | `needs-info`        | Waiting on reporter for more information |
| `ready-for-agent` | `ready-for-agent`   | Fully specified, ready for an AFK agent  |
| `ready-for-human` | `ready-for-human`   | Requires human implementation            |
| `wontfix`         | `wontfix`           | Will not be actioned                     |

`bug`, `enhancement`, and `wontfix` already exist as labels on this repo. `needs-triage`, `needs-info`, `ready-for-agent`, and `ready-for-human` do not exist yet — create them in GitHub (Settings → Labels) before the first `/triage` run; `/triage` does not create labels itself.

When a skill mentions a role (e.g. "apply the AFK-ready triage label"), use the corresponding label string from these tables.

**PRs as a request surface: no** (see `docs/agents/issue-tracker.md`) — `/triage` only walks issues in this repo, not external PRs.
