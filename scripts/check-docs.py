#!/usr/bin/env python3
"""Check tracked Markdown encoding, including archived documentation."""
from pathlib import Path
import subprocess
import sys

root = Path(__file__).resolve().parent.parent
files = subprocess.check_output(["git", "ls-files", "-z", "--", "*.md"], cwd=root).decode().split("\0")
errors = []
for filename in filter(None, files):
    path = root / filename
    if not path.exists():
        continue
    try:
        text = path.read_text(encoding="utf-8")
    except UnicodeError:
        errors.append(filename + ": invalid UTF-8")
        continue
    if "\ufffd" in text:
        errors.append(filename + ": contains Unicode replacement characters")
if errors:
    print("\n".join(errors), file=sys.stderr)
    sys.exit(1)
print("PASS: tracked Markdown uses UTF-8 without replacement characters")
