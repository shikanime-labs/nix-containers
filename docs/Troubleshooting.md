<!-- owner: shikanime | zone: internal | purpose: known failure modes and the first-responder fix for each -->

# Troubleshooting

## Push succeeds but only one arch is present

**Symptom:** a multi-platform build leaves a single-arch image, not an index.
**Cause:** `PLATFORMS`/`--platforms` was unset, so it defaulted to host arch.
**Fix:** set `PLATFORMS=linux/amd64,linux/arm64` (or `--platforms`) and confirm
`PUSH_IMAGE=true`; the index is written only after every platform image pushes.

## `containerd` push differs from `docker`

**Symptom:** image lands under one runtime but not the other, or tags diverge.
**Cause:** a change touched push/tag logic without exercising both clients.
**Fix:** run the build against **both** `docker` and `containerd` (per
`AGENTS.md`); keep `container.go` parity.

## Pure-eval rejects the flake

**Symptom:** build fails with an impure-eval error. **Cause:** the flake reads
external state. **Fix:** pass `--no-pure-eval` (`NO_PURE_EVAL=true`), or
`--accept-flake-config` (`ACCEPT_FLAKE_CONFIG=true`) to accept the flake config.

## Offline build fetches from the proxy

**Symptom:** `go build` hits the network despite a populated `vendor/`.
**Cause:** `flake.nix` uses `vendorHash = null`, so `buildGoModule` resolves
deps from the proxy, not `vendor/`. **Fix:** expected behaviour; if you must use
`vendor/`, set a real `vendorHash` and commit `vendor/`.
