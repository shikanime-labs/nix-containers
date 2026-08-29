<!-- owner: shikanime | zone: internal | purpose: how to consume, release, and protect the repo without surprises -->

# Runbook

This repo is a CLI library, not a deployable service. "Operations" means keeping
the binary installable and the fleet's image builds green.

## Consuming the binary

- Go install:

  ```bash
  go install github.com/shikanime-studio/nix-containers@latest
  ```

- From source: `go build -o nix-containers .`

- As a Skaffold custom builder: point `buildCommand` at
  `./nix-containers skaffold build --accept-flake-config` and set `IMAGE`,
  `BUILD_CONTEXT`, `PLATFORMS`, `PUSH_IMAGE` in the Skaffold environment.

## Releasing

There is no tag or publish step beyond the Go module proxy: consumers pin
`github.com/shikanime-studio/nix-containers@<rev>`. A change is "released" the
moment it merges to `main`; downstream `go install @latest` pulls it in.

## Branch protection

`main` requires one approving review, linear history, signed commits, and
squash+rebase only. PRs are the merge path; direct pushes are rejected.

## CI

`.github/workflows/` runs the format/eval pass on every PR, plus Renovate for
flake input bumps. Land bumps on `main` via squash+rebase (see `AGENTS.md`).
