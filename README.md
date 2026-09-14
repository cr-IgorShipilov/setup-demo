# setup (demo build)

A single-binary Go CLI that standardizes provisioning/deployment via Ansible:
generates inventory/config files, interactively collects server network
settings, validates target servers, and deploys with live per-host status.

> **This is a demo build.** `--check` and `--deploy` use fake, deterministic
> data instead of talking to real servers or running real Ansible. See
> [What's faked](#whats-faked-in-this-demo) below for exactly what to
> replace for a production build.

## Build

```bash
go build -o setup .
```

No external Go modules are required — the `go.mod` has zero dependencies.
The result is a single static binary; copy it anywhere and run it.

## Usage

Exactly one action flag must be given per invocation:

```
setup --init         (-i)    Generate demo hosts.ini and empty servers.yml
setup --configure    (-cfg)  Interactively configure 5 servers into servers.yml
setup --check        (-c)    Verify hardware/software prerequisites
setup --deploy       (-d)    Deploy to the environment with live status
```

Passing zero or more than one action flag prints usage and exits non-zero.

### `--init`

Creates `hosts.ini` (a demo Ansible inventory) and `servers.yml` (an empty
`servers: []` template) in the current directory. If either file already
exists, you'll be asked to confirm before it's overwritten.

### `--configure`

Interactively prompts for exactly 5 servers (`fqdn`, `ip`, `netmask`, `gw`,
`dns1`, `dns2`). Blank answers are rejected and re-prompted. Writes the
result to `servers.yml`, overwriting any existing content.

### `--check`

Reads `servers.yml` and prints a table of hardware/software prerequisite
results per server (CPU cores, RAM, disk, reachability, Python). If
`servers.yml` is missing or empty, it falls back to built-in demo data so
there's something to look at immediately after cloning the repo.

### `--deploy`

Simulates running an Ansible deployment against the configured servers,
printing live per-host/per-task status as it "runs," then a summary and a
non-zero exit code if anything failed.

### Logs

Every invocation writes a timestamped log file to `logs/<action>_<timestamp>.log`,
capturing what ran and its result — including detail not shown on screen
(e.g. structured per-event data during `--deploy`).

## What's faked in this demo

| Area | Demo behavior | Production behavior |
|---|---|---|
| `--check` facts | Deterministically derived from a hash of each server's FQDN (`internal/actions/check.go: fakeFactsFor`) | Real Ansible fact-gathering / ad-hoc modules against each host |
| `--deploy` events | Generated in-process on a Go channel (`internal/actions/deploy.go: emitFakeEvents`), with a scripted failure on the last host so both pass/fail rendering paths are visible | A custom Ansible callback plugin (Python) emitting JSON events over a named pipe (FIFO), consumed by the CLI in real time — avoids fragile stdout parsing while keeping Ansible itself untouched |
| `ansible-playbook` invocation | Not invoked at all | `os/exec` launches `ansible-playbook` as a subprocess with `ANSIBLE_CALLBACK_PLUGINS` pointed at the custom plugin |

## Known limitations (v1 scope, by design)

- **Linux/macOS only.** The production event-streaming design relies on a
  FIFO, which isn't available on Windows without additional work.
- **Hand-rolled `servers.yml` parser**, not a full YAML library. The schema
  is small and fixed for v1; if it needs to grow, migrate to a real YAML
  library (e.g. `gopkg.in/yaml.v3`) rather than extending the hand-rolled
  parser indefinitely.
- **Single environment per run.** No multi-inventory/multi-environment
  management in v1.
- **Fixed count of 5 servers** in `--configure`. Not currently configurable.

## Project layout

```
setup-demo/
├── main.go                        # flag parsing, dispatch, exactly-one-action enforcement
├── internal/
│   ├── actions/
│   │   ├── init.go                # --init
│   │   ├── configure.go           # --configure
│   │   ├── check.go                # --check (fake facts)
│   │   └── deploy.go              # --deploy (fake live events)
│   ├── config/
│   │   └── servers.go             # Server struct + hand-rolled servers.yml read/write
│   └── logging/
│       └── logger.go              # timestamped file + screen logging
└── go.mod
```
