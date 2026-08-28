---
name: sync-jonbaldie-skills
description: Install or update the jonbaldie/skills and mattpocock/skills collections in a project.
disable-model-invocation: true
---

Install both collections into one project's canonical Agent Skills directory.

1. Use the directory named by the user, or the current working directory when none is named. Resolve it to an absolute path.

2. Resolve the bundled script relative to this `SKILL.md`, then run it immediately:

```bash
"<this-skill-directory>/scripts/sync-skills.sh" "/absolute/path/to/project"
```

It installs both collections into `<project>/.agents/skills`, replaces previously managed skills with their current versions, and leaves unrelated skills alone. Use its output to report the two source commit IDs and the canonical destination. Do not inspect agent harness folders or reconstruct the installation yourself.

3. Only after the canonical install succeeds, ask whether the user also wants the managed skills copied to `<project>/.claude/skills`, `<project>/.gemini/skills`, or any other skill directories. Stop and wait for their answer.

4. If they choose additional directories, run:

```bash
"<this-skill-directory>/scripts/copy-to-skill-dirs.sh" "/absolute/path/to/project" ".claude/skills" ".gemini/skills"
```

Pass only the directories they chose. Relative paths resolve beneath the project; absolute paths remain absolute. The script copies only skills managed by the canonical installer and leaves unrelated entries alone.
