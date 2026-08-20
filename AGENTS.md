# Nix Containers

Build OCI images from Nix flakes, with optional multi-platform output and push
to registries. Designed for use as a Skaffold custom builder, but usable
directly from the CLI.

**Language:** Go (Nix for build definitions)

## Structure

- `main.go` — CLI entry point
- `cmd/` — Subcommand implementations
- `nix/` — Nix build logic
- `flake.nix` — Development shell

## Usage

Build container images directly from Nix flakes. Supports multi-platform builds
and pushing to container registries. Integrates with Skaffold as a custom
builder.

## Commit Style

- Plain-text capitalized title, no conventional-commit prefix
- Body with labels: `Design:`, `Related:`, `Closes #`
- Keep Markdown lines wrapped at 80 columns and run `nix fmt` before shipping

## Stack

- The commit title **is** the PR title; the commit body **is** the PR body
- Split work into stacked PRs to keep each PR small and reviewable
- To update a PR: edit files, then `jj squash` (or `git commit --amend`) into
  the **target commit** of the stack — the one that PR represents; the commit
  message updates the PR title and body automatically when resubmitted
- Never `gh pr merge` (creates poisoned commits)

## Protect `main`

- Require 1 approving review
- Require linear history (no merge commits)
- Require signed commits
- Squash+rebase merge only

_Licensed under AGPL-3.0. Test with both `docker` and `containerd` runtimes.
Always use worktrees when making changes._

## Stack Workflow

- Install the official GitHub extension once:
  `gh extension install github/gh-stack` (requires GitHub CLI ≥ 2.0; `gh stack`
  is in public preview and may change).
- Keep one logical change per PR; split large work into a stack of PRs.
- Create a stack: `gh stack init`, then `gh stack add` for each new branch, and
  commit on the active branch. `gh stack view` lists the stack.
- Submit/update: `gh stack submit` (add `--open` to open PRs, `--auto` to skip
  prompts). Resubmit after each change to refresh titles, bodies, and branches.
- Pull down an existing stack: `gh stack checkout <PR_NUMBER>` (also accepts a
  stack number, PR URL, or branch name).
- Rebase onto updated trunk: `gh stack rebase` (cascading), then
  `gh stack submit`.
- Land a stack: `gh stack merge` (interactive) or
  `gh stack merge <PR_NUMBER> --yes --squash` to merge up to a PR.
- Never `gh pr merge` on a stacked PR — only `gh stack merge` lands stacks.
- Never force-push stack branches; `gh stack` owns the branch pointers.
