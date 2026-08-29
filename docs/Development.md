<!-- owner: shikanime | zone: internal | purpose: the local build/format/test loop and how to extend the builder -->

# Development

## Prerequisites

- A recent Nix with flakes enabled (drives `nix fmt` and the dev shell).
- Go toolchain (the binary is Go; `go` on PATH, or use the flake dev shell).
- `direnv` (the repo ships `.envrc`); `direnv allow` to load the dev shell.
- This is a `jj` repo. Branch off `main`; never commit to `main` directly.

## Build and check loop

```bash
nix fmt                       # treefmt: Go + Nix formatting, markdown lint
go build -o nix-containers .  # build the binary
go test ./...                 # run Go unit tests (incl. nix_test.go)
```

CI (`.github/workflows/`) runs the format/eval pass on every PR. `nix fmt` must
be clean before a PR is reviewable.

## Commit style

Plain capitalized title, no conventional-commit prefix. Body uses labels:

```text
Design: <why the builder behaviour changed>
Related: <full URL to issue/PR>
Closes: <full URL>
```

Keep Markdown wrapped at 80 columns. `nix fmt` enforces it.

## Dependency note

`flake.nix` sets `vendorHash = null` and builds offline via `buildGoModule`;
dependencies resolve from the Go proxy, not the committed `vendor/` tree. Bump
versions through `go.mod`, re-run `go mod tidy`, then `nix fmt`.

## Adding a feature

1. Implement it under `cmd/nix-containers/` (keep `builder.go` the spine).
2. Preserve `docker` + `containerd` parity in `container.go`.
3. Add a unit test beside the change (e.g. `nix_test.go`).
4. `nix fmt`, `go test ./...`, open a PR against `main`.
