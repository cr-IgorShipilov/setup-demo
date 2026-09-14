# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`setup` is a single-binary Go CLI that standardizes Ansible-based provisioning/deployment.
**This repo is a demo build**: `--check` and `--deploy` produce fake, deterministic data
instead of talking to real servers or invoking Ansible. Keep that framing when changing
code — the point of the demo is to exercise the CLI's structure, file formats, and
live-status UX, so changes should preserve the shape of the real design rather than
quietly turning fakes into real infrastructure calls.

## Commands

```bash
go build -o setup .      # build (zero external modules; go.mod has no deps)
go vet ./...
gofmt -l .               # should print nothing

./setup --init           # -i    write demo hosts.ini + empty servers.yml
./setup --configure      # -cfg  interactive prompts -> servers.yml
./setup --check          # -c    prerequisite table (fake facts)
./setup --deploy         # -d    simulated deploy with live per-host status
```

Exactly one action flag per invocation; zero or two+ prints usage and exits 1.

There are no tests in the repo yet. If you add any: `go test ./...`, single test with
`go test ./internal/actions -run TestName`.

Running any action writes `logs/<action>_<timestamp>.log` and, for `--init`/`--configure`,
`hosts.ini` / `servers.yml` into the **current working directory**. Run throwaway
invocations from a temp dir to avoid littering the repo.

## Architecture

`main.go` owns all flag parsing and the exactly-one-action rule. `selectAction` maps the
chosen flag to `(name string, func(*logging.Logger) error)`; the path constants
(`hosts.ini`, `servers.yml`, `logs`) live only in `main.go` and are injected into actions
as arguments. Actions never read globals for paths — keep that, it's what makes them
testable. `main.go` also owns the exit code: an action returning an error means exit 1
plus a pointer to the log file.

Three packages under `internal/`:

- **`config`** — the `Server` struct (`fqdn, ip, netmask, gw, dns1, dns2`) plus
  hand-rolled line-based readers/writers. `Save` and `Load` are a matched pair for a
  fixed schema; changing one without the other silently breaks round-tripping.
  `ParseHostsINI` extracts `<fqdn> ansible_host=<ip>` lines to pre-fill `--configure`
  defaults. A real YAML library is deliberately avoided for v1 — if the schema needs to
  grow beyond these six flat fields, migrate to `gopkg.in/yaml.v3` rather than extending
  the parser.
- **`logging`** — the two-channel convention every action depends on: `Printf` writes to
  **screen and file**, `Infof` writes to the **file only**. Per-event/per-check detail
  goes through `Infof` so the log is richer than the terminal; user-facing progress goes
  through `Printf`. Note that `check.go`/`deploy.go` also call `fmt.Printf` directly for
  table and status rows — those are screen-only by design, with the structured
  equivalent logged separately via `Infof`.
- **`actions`** — one file per action, each a single exported function.

`--check` and `--deploy` both call `config.Load` and fall back to `config.FakeServers()`
when the file is missing or empty, so the tool always has something to show on a fresh
clone. Preserve that fallback.

## The fake/real seams

These three points are where a production build would differ. Keep them isolated and
clearly commented:

| Seam | Demo | Production |
|---|---|---|
| `actions/check.go: fakeFactsFor` | FNV hash of FQDN → stable plausible facts | real Ansible fact gathering |
| `actions/deploy.go: emitFakeEvents` | goroutine feeding an in-process `chan event` | Ansible callback plugin emitting JSON over a FIFO |
| `ansible-playbook` | never invoked | `os/exec` with `ANSIBLE_CALLBACK_PLUGINS` set |

`Deploy` consumes `event`s from a channel and renders them as they arrive — that consumer
loop is the part meant to survive into production unchanged, so don't collapse it into a
synchronous loop. `emitFakeEvents` intentionally sleeps 150ms per task and scripts a
failure on the **last** host's "Verify health endpoint" task so both the success and
failure rendering paths (and the non-zero exit) are visible in a demo run.

## Conventions

- Linux/macOS only by design (the production FIFO approach rules out Windows).
- `--check` colorizes only the `STATUS` column (via `colorStatus`), which is the last
  tab-separated field, so ANSI escapes don't corrupt `tabwriter` column widths. If you
  add columns, keep the colorized one last or the table will misalign.
- `--configure` writes `servers.yml` unconditionally; `--init` asks before overwriting
  either file.

## Known drift

`serverCount` in `actions/configure.go` is **3**, but `README.md` and `main.go`'s usage
text disagree (README says 5, `printUsage` says 3). Treat the constant as the source of
truth and fix the docs if you touch this area.
