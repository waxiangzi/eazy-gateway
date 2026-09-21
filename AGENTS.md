## Agent skills

### Issue tracker

Issues live as GitHub Issues in `waxiangzi/eazy-gateway`, managed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default five-role vocabulary (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`), used as-is. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout — `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.

### CodeGraph worktrees

Using `git worktree` here: the CodeGraph index resolves upward, so a worktree nested in the main checkout answers from another branch. See `docs/agents/codegraph-worktrees.md`.
