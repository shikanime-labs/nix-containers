<!-- owner: shikanime | zone: internal | purpose: explain the command/module tree and the Nix-build path so changes land in the right file -->

# Architecture

## Goal

Build OCI images from Nix flakes without hand-written Dockerfiles. The tool
evaluates a flake that produces a container image derivation, runs the Nix
build, and ships the result to a registry — either single-platform or a
multi-arch index. The design constraint is **runtime portability**: the same
binary must drive both the `docker` and `containerd` clients, and the same build
must serve Skaffold and the bare CLI.

## Command tree

```text
main.go                    # root + build/skaffold command wiring
cmd/nix-containers/
  main.go                  # CLI entry, flag parsing
  skaffold.go              # Skaffold custom-builder entry (reads BUILD_CONTEXT)
  builder.go               # orchestrates build + push, platform fan-out
  container.go             # container runtime client (docker/containerd)
  nix.go                   # Nix build evaluation + image load
  nix_test.go              # Nix build unit tests
  tracing.go               # OpenTelemetry tracing hooks
```

`cmd/nix-containers/builder.go` is the spine: it takes the image reference and
platforms, calls `nix.go` to realize the derivation, then hands each realized
image to `container.go` for tagging/pushing and assembles the multi-arch index.

## Build path

1. Evaluate the flake at `BUILD_CONTEXT` (pure eval unless `--no-pure-eval`).
2. `nix.go` builds the image attr and loads it into the local image store.
3. `builder.go` fans the realized image out across `--platforms`.
4. `container.go` pushes each platform image, then writes the manifest index.

## Skaffold bridge

`skaffold.go` implements the custom-builder contract: Skaffold sets `IMAGE` and
`BUILD_CONTEXT` and invokes `nix-containers skaffold build`. The CLI path and
the Skaffold path share `builder.go` exactly — the only difference is where the
context and image come from (env vs args).

## Parity rule

Any change to push or tag logic must work against **both** the `docker` and
`containerd` clients in `container.go`; a fix that only one runtime exercises is
a bug, not a feature. Test both.
