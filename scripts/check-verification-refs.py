#!/usr/bin/env python3
"""Reject verification-guide commit references that a squash merge would lose."""

import argparse
import re
import subprocess
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("base", help="the target branch, e.g. origin/main")
    parser.add_argument("--guides", type=Path, default=Path(".agents/skills"))
    args = parser.parse_args()
    subprocess.run(["git", "rev-parse", "--verify", f"{args.base}^{{commit}}"], check=True, stdout=subprocess.DEVNULL)
    guides = sorted(args.guides.glob("verify-*/SKILL.md"))
    if not guides:
        parser.error(f"no verification guides found under {args.guides}")
    failed = False
    for guide in guides:
        for line_no, line in enumerate(guide.read_text().splitlines(), 1):
            for ref in re.findall(r"(?<![a-zA-Z0-9])[0-9a-f]{7,40}(?![a-zA-Z0-9])", line):
                result = subprocess.run(["git", "merge-base", "--is-ancestor", ref, args.base], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                if result.returncode:
                    print(f"{guide}:{line_no}: {ref} is not an ancestor of {args.base}; use a verified commit reachable from the target branch")
                    failed = True
    return int(failed)


if __name__ == "__main__":
    raise SystemExit(main())
