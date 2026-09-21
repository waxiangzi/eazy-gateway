# CodeGraph Worktrees

Keeping the CodeGraph index honest while using `git worktree` in this repo.

## How the index is found

CodeGraph keeps its index in `<checkout>/.codegraph/` and resolves it by walking up from the current directory to the nearest `.codegraph/`. Two consequences:

- Each worktree holds its own `codegraph.db` and its own daemon (`daemon.sock` sits inside that same directory; the daemon exits after 300s idle). `~/.codegraph/` carries CLI versions only — never an index.
- A worktree created *inside* the main checkout — a gitignored path such as `.claude/worktrees/<name>/` — walks up and lands on the **main checkout's** index.

## The borrowed-index warning

Queries run from a nested worktree answer from the main checkout's graph, often a different branch, so symbols that exist only in the worktree are invisible. CodeGraph detects this and says so:

```
⚠ CodeGraph results below come from a different git worktree (<indexRoot>), not where
you're working (<worktreeRoot>) — they may reflect another branch, and symbols changed
only here are missing.
```

`codegraph status` prints the long form; `status --json` fills `worktreeMismatch: {worktreeRoot, indexRoot}` and leaves it `null` when the resolved index is the right one. CodeGraph read tools prefix their results with the one-line notice.

Give the worktree its own index — `codegraph init` in that directory — and query again. (`init -i` in the tool's own wording is the older spelling of the same thing; indexing runs by default now.)

## Working rules

- Create worktrees as siblings of the main checkout (`git worktree add ../eazy-gateway-<topic>`), then run `codegraph init` there.
- After a branch switch, a merge, or a batch of edits, run `codegraph sync`. The index is a content snapshot and does not follow branches, so stale `file:line` positions are the failure to watch for; `status --json`'s `pendingChanges` counts what the index has not seen yet.
- To query another worktree's index deliberately, pass `projectPath` to the CodeGraph tools.
- When the worktree is done: `codegraph uninit`, then `git worktree remove`.

## Cleanup and gitignore

The repo root ignores `.codegraph/` (beside `.omo/`). A worktree created from an older commit can predate that rule — confirm with `git check-ignore -v .codegraph/codegraph.db`.

Without the rule, the tool's own `.codegraph/.gitignore` appears as untracked and `git worktree remove` refuses with exit 128 (`contains modified or untracked files, use --force`). Running `codegraph uninit` first removes the index directory, after which the plain removal succeeds. With the rule in place the removal succeeds as-is.

When the CodeGraph git sync hook is installed it lives in the shared hooks directory (`git rev-parse --git-path hooks`), so one install covers every worktree of this repository. The hook runs `codegraph sync` in its own working directory, i.e. the worktree that just committed.
