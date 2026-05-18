---
description: Regenerate rules and libs cheatsheets from guide files
---

# Refresh Rules & Libs Cheatsheets

Regenerate `.claude/guides/rules_cheatsheet.md` and `.claude/guides/libs_cheatsheet.md` from all guide and lib files.

## Steps

1. Regenerate rules cheatsheet:

```bash
python3 .claude/scripts/extract_rules.py cheatsheet --output .claude/guides/rules_cheatsheet.md
```

2. Regenerate libs cheatsheet:

```bash
python3 .claude/scripts/extract_rules.py libs-cheatsheet --output .claude/guides/libs_cheatsheet.md
```

3. Report the result to the user (number of rules and lib patterns written).
