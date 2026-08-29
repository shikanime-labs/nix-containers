<!-- owner: shikanime | zone: internal | purpose: the CLI surface (commands, flags, env vars) for consumers -->

# Reference

## Commands

| Command          | Effect                         |
| ---------------- | ------------------------------ |
| `build <ctx>`    | Build (push if `PUSH_IMAGE`).  |
| `skaffold build` | Skaffold custom builder entry. |

## Global flags

| Flag                    | Env                   | Effect               |
| ----------------------- | --------------------- | -------------------- |
| `--accept-flake-config` | `ACCEPT_FLAKE_CONFIG` | Accept flake config. |

## `build` flags

| Flag             | Env            | Effect                         |
| ---------------- | -------------- | ------------------------------ |
| `--no-pure-eval` | `NO_PURE_EVAL` | Disable pure eval.             |
| `--platforms`    | `PLATFORMS`    | `os/arch` list; overrides env. |

## Environment variables

| Variable              | Req | Effect                                     |
| --------------------- | --- | ------------------------------------------ |
| `IMAGE`               | yes | Target ref, e.g. `ghcr.io/you/app:latest`. |
| `PLATFORMS`           | no  | `os/arch` list; host arch default.         |
| `BUILD_CONTEXT`       | no* | Flake path; needed by `skaffold build`.    |
| `PUSH_IMAGE`          | no  | `true                                      | ...` — push after build. |
| `LOG_LEVEL`           | no  | `info                                      | debug                    | warn | error`(default`info`). |
| `ACCEPT_FLAKE_CONFIG` | no  | Accept flake config.                       |

\* For `build`, pass the context as a positional argument instead.

## Auth

Registry auth uses Docker credential helpers via the default keychain. When
building multi-platform images with push enabled, each platform image is pushed
first, then the multi-arch index is written.
