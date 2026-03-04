# Branch scope drift when accumulating unrelated work

`task/join-account-flag` started as a single feature branch but accumulated 17 commits spanning the `--account` flag, company loop migration, spec cleanup, and plan housekeeping.
The PR title was updated but the branch name stayed stale.

**Pattern:** when a branch accumulates work beyond its original scope, either:
1. Merge and start fresh before the new work begins
2. At minimum, update the PR title/description (done here, but late)

**Signal:** if you're about to commit something unrelated to the branch name, that's the moment to ask about splitting.
