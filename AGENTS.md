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

## Commit Style

- Plain-text capitalized title, no conventional-commit prefix
- Body with labels: `Design:`, `Related:`, `Closes #`
- Keep Markdown lines wrapped at 80 columns and run `nix fmt` before shipping

## Stack

- One feature/fix == one branch == one PR, opened with `gh pr create`.
- Re-submit by amending the branch and force-pushing the feature branch
  (never rewrite `main`).
- Land via `gh pr merge --squash` once review approval is in.
- Keep history linear: no merge commits into `main`.

## Protect `main`

- Require 1 approving review
- Require linear history (no merge commits)
- Require signed commits
- Squash+rebase merge only

_Licensed under AGPL-3.0. Test with both `docker` and `containerd` runtimes_
