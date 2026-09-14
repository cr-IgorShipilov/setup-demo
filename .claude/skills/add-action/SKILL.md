---
name: add-action
description: Add a new action to the `setup` CLI in this repo — a fifth command alongside --init/--configure/--check/--deploy. Use this whenever someone asks to add, wire up, scaffold, or implement a new command, flag, subcommand, mode, or action for the setup tool, including indirect phrasings like "add a --status flag", "I want a rollback command", "make setup able to also tear things down", or "setup should have a way to show the last deploy". Covers the four wiring points in main.go that must move together, the actions package contract, the Printf/Infof logging split, and the fake-data seam that keeps this a demo build.
---

# Adding an action to `setup`

An "action" is one top-level mode of the CLI: `--init`, `--configure`, `--check`,
`--deploy`. Exactly one runs per invocation. Adding a fifth is mostly mechanical,
but the mechanics are spread across four spots in `main.go` plus a new file, and
missing any one of them fails in a way the compiler will not catch. That is what
this skill is for.

Read `main.go` and one existing action (`internal/actions/check.go` is the most
representative) before writing anything. The codebase is ~800 lines total and the
conventions are visible in it; this document tells you which parts are load-bearing.

## Step 1: Pick the name and shorthand

Long flag is the action name (`--status`). Shorthand is whatever short form is
still free. Currently taken: `i`, `cfg`, `c`, `d`, plus the long names themselves.

Duplicates matter more than usual here. Go's `flag` package **panics at
registration** on a redefined flag, so a collision compiles cleanly and then kills
the binary on every single invocation, including `--help`. Grep before you commit
to a letter:

```bash
grep -n 'flag.BoolVar' main.go
```

Go treats `-status` and `--status` identically, so there is no separate work to
support both spellings.

## Step 2: Wire it into main.go — four places, all or nothing

These four edits are a set. The compiler enforces none of them, and each one fails
differently, so it is worth knowing what a miss looks like.

**1. Declare the variable** in the `var (...)` block at the top of `main`:

```go
var (
    doInit      bool
    doConfigure bool
    doCheck     bool
    doDeploy    bool
    doStatus    bool // new
)
```

**2. Register both flags**, long then shorthand. The shorthand's description is
always `"Shorthand for --<long>"` — the usage text is hand-written in `printUsage`,
so these strings only surface via `flag`'s own fallback output:

```go
flag.BoolVar(&doStatus, "status", false, "Show the result of the last deployment")
flag.BoolVar(&doStatus, "s", false, "Shorthand for --status")
```

**3. Add it to the `countTrue` call.** This is the one that bites:

```go
selected := countTrue(doInit, doConfigure, doCheck, doDeploy, doStatus)
```

`countTrue` is variadic, so forgetting the new argument compiles fine. The symptom
is that `setup --status` counts zero selected actions, prints usage, and exits 1 —
looking for all the world like the flag never registered.

**4. Add a `case` to `selectAction`.** Extend the signature and add an explicit
case. Note that the existing `default:` branch *is* deploy — do not add your action
as the new default, or `--deploy` silently starts running your code:

```go
func selectAction(doInit, doConfigure, doCheck, doDeploy, doStatus bool) (string, func(*logging.Logger) error) {
    switch {
    case doInit:
        ...
    case doStatus:
        return "status", func(log *logging.Logger) error {
            return actions.Status(serversPath, log)
        }
    default: // doDeploy
        ...
    }
}
```

The returned name string becomes the log filename (`logs/status_<timestamp>.log`),
so keep it lowercase and matching the flag.

**Then update `printUsage`** in the same file. It is a hand-aligned raw string, so
match the existing column positions by eye rather than trusting a formatter:

```
  setup --status       (-s)    Show the result of the last deployment
```

`README.md` carries the same list. Update it too — the repo already has one
documented drift between the code and the docs, and adding a second is not a good
trade.

## Step 3: Write the action file

One file per action, one exported function, at `internal/actions/<name>.go`:

```go
// Status implements "setup --status": it reads servers.yml (falling back to
// demo data when missing/empty) and prints the last recorded deployment
// result per server.
func Status(serversPath string, log *logging.Logger) error {
```

Two conventions in that signature are worth protecting:

**Paths arrive as arguments.** The constants `hostsPath`, `serversPath`, and
`logDir` live only in `main.go`. Actions never reach for a global or hardcode a
filename — that is what lets an action be called against a temp directory in a
test. If your action needs a new path, add the constant to `main.go` and pass it
in; do not introduce a package-level path in `actions`.

**The doc comment names the flag.** Every action opens with
`// Name implements "setup --flag": it ...`. It is a small thing, but it is how
someone reading the package finds the entry point from the CLI surface.

If the action reads server config, mirror the existing fallback so a fresh clone
always has something to show:

```go
servers, err := config.Load(serversPath)
if err != nil {
    return fmt.Errorf("load servers.yml: %w", err)
}
if len(servers) == 0 {
    servers = config.FakeServers()
    log.Printf("No servers found in %s — showing demo data.", serversPath)
}
```

Returning an error from the action means exit code 1 plus a pointer to the log
file; `main.go` handles both. Do not call `os.Exit` from an action.

## Step 4: Use the two logging channels deliberately

`logging.Logger` has a split that every action depends on:

- `log.Printf` → **screen and file**. User-facing progress.
- `log.Infof` → **file only**. Per-event, per-host, per-check detail.

The intent is that the log file is strictly richer than the terminal. A demo
audience watching the screen sees a clean narrative; whoever opens the log
afterwards sees every event behind it. When you emit a row of detail, send the
human-readable version to the screen and the structured version to `Infof`:

```go
fmt.Printf("%s\t%s\t%s\n", s.FQDN, check, status)          // screen only, by design
log.Infof("check host=%s requirement=%q status=%s", s.FQDN, check, status)
```

That bare `fmt.Printf` is not an oversight — `check.go` and `deploy.go` both do it
for table rows and live status lines, because those need `tabwriter` control or
carriage-return updates that a line-oriented logger would mangle. Screen-only
output is fine as long as the structured equivalent reaches the log.

## Step 5: Keep it a demo

This repo simulates. `--check` derives facts from an FNV hash of the FQDN;
`--deploy` feeds an in-process channel from a goroutine; `ansible-playbook` is
never invoked. A new action must not be the thing that quietly starts talking to
real infrastructure — no `os/exec` of Ansible, no SSH, no network calls.

Generate fake data the same way the existing seams do, and comment the seam so the
next reader knows where production would differ:

```go
// fakeStatusFor deterministically derives a plausible last-deploy result
// from the host's FQDN. In production this would read the deployment
// record written by the Ansible callback plugin.
```

Determinism matters for the demo specifically: hashing the FQDN means the same
host shows the same numbers on every run, so a live demo never surprises the
person giving it. Prefer `hash/fnv` over `math/rand` for that reason.

If your action streams events over time, consume them from a channel rather than
looping synchronously. The consumer loop in `deploy.go` is explicitly the part
meant to survive into production unchanged — in the real build the same loop reads
JSON from a FIFO fed by an Ansible callback plugin instead of from a goroutine.

## Step 6: Verify

The repo has no tests and zero module dependencies, so verification is these three
commands. All three must be clean:

```bash
go build -o setup .
go vet ./...
gofmt -l .          # must print nothing
```

Then exercise the action itself. Every run writes `logs/<action>_<timestamp>.log`
into the **current working directory**, and `--init`/`--configure` write
`hosts.ini` and `servers.yml` there too — so run from a temp directory to avoid
littering the repo:

```bash
go build -o /tmp/setup-check . && cd "$(mktemp -d)" && /tmp/setup-check --status
```

Check three things beyond "it ran": the shorthand works too (`-s`), two actions
together still fail (`--status --check` should print usage and exit 1), and the log
file exists and contains more detail than the screen showed.

## If you add a column to a table

`--check` colorizes only the `STATUS` column, and it gets away with that because
`STATUS` is the last tab-separated field — ANSI escape bytes inside a `tabwriter`
cell would otherwise be counted as display width and skew every column. If you add
columns to an existing table or build a new colorized one, keep the colorized field
last.
