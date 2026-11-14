#!/usr/bin/env python3
"""Fail if a repository-relative link in the documentation names a missing file.

Run from the repository root on a clean checkout (CI does), so a link to a file
that exists only locally, untracked or ignored, is reported as missing.
Checks CHANGELOG.md, README*.md and docs/**/*.md by default, or the files given.
"""
import pathlib
import re
import sys

LINK = re.compile(r"\]\(\s*<?([^)\s>]+)>?(?:\s+\"[^\"]*\")?\s*\)")
SKIP = ("http://", "https://", "mailto:", "#", "tel:")


def files(args):
    if args:
        return [pathlib.Path(a) for a in args]
    root = pathlib.Path(".")
    out = [p for p in [root / "CHANGELOG.md"] if p.exists()]
    out += sorted(root.glob("README*.md")) + sorted(root.glob("docs/**/*.md"))
    return out


def main(args):
    missing, checked = [], 0
    for doc in files(args):
        text = doc.read_text(encoding="utf-8")
        # Ignore fenced code blocks: they hold examples, not links.
        text = re.sub(r"```.*?```", "", text, flags=re.S)
        for m in LINK.finditer(text):
            target = m.group(1)
            if target.startswith(SKIP):
                continue
            path = target.split("#", 1)[0]
            if not path:
                continue
            checked += 1
            if not (doc.parent / path).exists():
                missing.append(f"{doc}: {target}")
    for m in missing:
        print("missing link target:", m)
    print(f"checked {checked} repository-relative links")
    return 1 if missing else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
