<!-- owner: shikanime | zone: internal | purpose: docs landing + index for the nix-containers OCI builder repo -->

# nix-containers — Documentation

A Go CLI that builds OCI images from Nix flakes, with optional multi-platform
output and push to registries. It is designed as a Skaffold custom builder but
also runs standalone from the CLI. It ships no long-running service.

## Internal ops

- [Architecture](./Architecture.md) — command tree, the Nix-build path, and the
  multi-platform push flow.
- [Development](./Development.md) — local setup, the build/format/test loop, and
  how to extend the builder.
- [Runbook](./Runbook.md) — how to consume, release, and protect the repo.
- [Troubleshooting](./Troubleshooting.md) — runtime client, vendor, and
  pure-eval failures.
- [Reference](./Reference.md) — CLI flags, env vars, and commands.

## User-facing docs

The user guide lives in the repo [README](../README.md) (install, commands,
flags, Skaffold example). It is the canonical source for consumers; this `docs/`
directory owns internal ops only and links out rather than duplicating it.
