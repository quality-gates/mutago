---
name: sync-jonbaldie-skills
description: Update installed jonbaldie/skills and mattpocock/skills collections.
disable-model-invocation: true
---

Update existing global installations of `mattpocock/skills` and `jonbaldie/skills`.

1. Inspect `~/.agents/skills` and `~/.claude/skills`. Tell the user which exist and whether they resolve to the same directory. Ask whether they want any other global agent harness skill folders updated. If the invocation already names additional folders, treat that as the answer. Wait for the answer before changing any skill folder.

2. Use the existing default folders plus any existing folders the user supplies. Resolve symlinks and update each physical directory once.

3. Clone the current default branches of `https://github.com/mattpocock/skills.git` and `https://github.com/jonbaldie/skills.git` once each into a temporary directory. A source skill is a directory below the repository's `skills/` directory containing `SKILL.md`; its installed name is the frontmatter `name`.

4. For each chosen folder, consider a collection installed when at least one top-level skill directory has a name found in that collection. Update every skill from each installed collection with `rsync --archive --delete`, creating newly published skills and replacing stale files inside same-named directories. Apply Matt's collection first and Jonathan's second so Jonathan's version wins a name collision. Leave every other top-level entry alone.

5. Verify every updated skill with the equivalent of:

```bash
rsync --archive --checksum --dry-run --itemize-changes --delete \
  "${source}/" "${destination}/${name}/"
```

Require empty output, then report the source commit IDs, updated folders, and collections updated in each folder. Remove the temporary clones.
