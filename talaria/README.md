# talaria

**Disposable workspace manager.** 

```bash
eval "$(talaria shell-init bash)"   # bash | zsh | fish
eval "$(talaria shell-init bash)"   # once per shell session
talaria new "Investigate mmap behavior"
# → creates /tmp/talaria/y4f8, records it, prints you there
```

Works standalone (local JSON registry) or with an optional daemon (`talariad`) 

Optional hourly maintain via systemd user timer — **install units from the
toolkit / 
Unit files live in `tools/systemd-services/

## Quick start

```bash
talaria new "spike protobuf codecs"   # create + cd (with shell-init)
talaria ls                            # list (pinned first; DUR=D if durable)
talaria show y4f8                     # detail: tags, notes, timestamps
talaria pin y4f8                      # survive gc (not reboot — see perdure)
talaria perdure y4f8                  # move to durable disk + pin (survives reboot)
talaria new --durable "long job"      # create already durable + pinned
talaria tag y4f8 protobuf
talaria note y4f8 "try wire-format first"
talaria find protobuf
talaria enter y4f8                    # cd back (with shell-init)
talaria gc --older-than 7d            # drop old unpinned workspaces
talaria rm y4f8                       # delete registry entry + directory
```

Without shell integration, compose with the shell yourself:

```bash
cd "$(talaria new "Investigate mmap")"
cd "$(talaria enter y4f8)"
cd "$(talaria perdure y4f8)"
```

## Commands

| Command | Description |
|---------|-------------|
| `talaria new [desc]` | Create workspace; print path on stdout |
| `talaria new --durable [desc]` | Create under durable root and pin |
| `talaria ls` | List workspaces (pinned first; DUR column) |
| `talaria show <id>` | Detail view (tags, notes, timestamps) |
| `talaria enter <id>` | Print path + update `last_used` |
| `talaria rm <id>` | Delete registry entry + directory |
| `talaria pin` / `unpin <id>` | Survive / allow `gc` (not reboot) |
| `talaria perdure <id>` | Move to durable root + pin (survives reboot) |
| `talaria gc [--older-than 7d]` | Drop unpinned workspaces |
| `talaria tag` / `untag <id> <tag>` | Attach / detach a tag |
| `talaria find <tag>` | Workspaces with that tag |
| `talaria note <id> "…"` | Append a note |
| `talaria notes <id>` | List notes |
| `talaria maintain` | Reconcile missing dirs + age GC (timer oneshot) |
| `talaria status` | Report daemon vs local backend |
| `talaria doctor` | Diagnose socket, lock, registry, workspace roots |
| `talaria shell-init bash` | Emit wrapper function (`bash` \| `zsh` \| `fish`) |

Global flags:

| Flag | Effect |
|------|--------|
| (default) | Prefer daemon; fall back to local |
| `--local` / `TALARIA_FORCE_LOCAL=1` | Always local |
| `--require-daemon` / `TALARIA_FORCE_DAEMON=1` | Fail if daemon is down |

```bash
talaria --help
talaria gc --help
talaria completion bash   # shell completion (EnableShellCompletion)
```

## Layout

```
~/.local/share/talaria/
    workspaces.json        # registry (atomic rewrite)
    workspaces.json.lock   # exclusive flock
    talaria.sock           # fallback socket path
    talaria.pid            # daemon PID (next to socket)

$XDG_RUNTIME_DIR/talaria.sock   # preferred socket when set
$XDG_RUNTIME_DIR/talaria.pid

/tmp/talaria/              # ephemeral (or $TMPDIR/talaria)
    y4f8/
    pq81/

~/.local/share/talaria/workspaces/   # durable (perdure / new --durable)
    ab12/
```

| Variable | Purpose |
|----------|---------|
| `TALARIA_DATA_DIR` | State directory (registry + lock) |
| `TALARIA_WORKSPACE_ROOT` | Ephemeral workspace folders (default `/tmp/talaria`) |
| `TALARIA_DURABLE_ROOT` | Reboot-safe folders (default `<data-dir>/workspaces`) |
| `TALARIA_SOCKET` | Unix socket path for `talariad` |

### Pin vs perdure

| | Survives `gc` / maintain age GC | Survives reboot / tmp wipe |
|--|--------------------------------|----------------------------|
| **pin** | yes | **no** if path is under `/tmp` |
| **perdure** | yes (pins by default) | **yes** (moves files off volatile storage) |

`talaria doctor` warns about pinned workspaces that still live under a volatile path.

### Backend resolution

```
talaria new …
        │
        ▼
  dial talariad socket ──success──► Remote backend (daemon owns registry)
        │
     failure
        │
        ▼
  Local backend (open JSON + flock)
```

### File locking

- **Local CLI**: exclusive flock for the duration of one command (`Open` → `Close`).
- **Daemon**: holds the flock for its entire lifetime (sole long-lived writer).
- Local fallback while the daemon is up will **not** steal the lock (bounded wait, then error).

## Daemon (`talariad`)

Optional long-lived process for low-latency multi-client access. The CLI still works without it via local fallback.

```bash
talariad                              # listen + own registry
talariad --gc-interval 1m --gc-older-than 7d
talaria status                        # Backend: daemon
talaria doctor                        # socket, lock PID, registry, workspace root
```

- Opens the registry once and holds the flock for its lifetime.
- Clients speak line-delimited JSON over the Unix socket.
- Writes a **PID file** next to the socket (`talaria.sock` → `talaria.pid`) on start; removes it on exit.
- Background GC ticker is context-cancelled with the process.
- Graceful shutdown on `SIGINT` / `SIGTERM`.

### Doctor and stale daemon

`talaria doctor` reports:

| Check | What it verifies |
|-------|------------------|
| socket | Unix socket present and pingable |
| lock | Exclusive flock free or held (PID from pid file and/or `fuser`/`lsof`) |
| registry | `workspaces.json` parses as JSON |
| workspace-root | Directory exists and is writable |

If `talariad` crashes while holding the lock, or leaves a stale socket:

```bash
# Who holds the registry lock?
fuser -v ~/.local/share/talaria/workspaces.json.lock
lsof   ~/.local/share/talaria/workspaces.json.lock

# Kill stale talariad (prefer the PID file when present)
kill $(cat "${XDG_RUNTIME_DIR:-$HOME/.local/share/talaria}/talaria.pid")
# or: kill <pid-from-fuser>

# Orphaned socket/pid after a hard kill
rm -f "${XDG_RUNTIME_DIR:-$HOME/.local/share/talaria}/talaria.sock" \
      "${XDG_RUNTIME_DIR:-$HOME/.local/share/talaria}/talaria.pid"
```

`doctor` prints the same `fuser` / `lsof` / `kill` hints when it detects an inconsistent lock or dead PID file.

## Hourly maintain (systemd)

Wake-on-call cleanup: **timer wakes → oneshot runs → exits**. No long-running process required for scheduled GC.

From the toolkit workspace (after `make install` put `talaria` on `install_dir`):

```bash
make install-systemd   # toolkit target — copies talaria/tools/systemd-services/*
systemctl --user daemon-reload
systemctl --user enable --now talaria-maintain.timer
```

| Unit | Behaviour |
|------|-----------|
| `talaria-maintain.timer` | `OnBootSec=1min`, `OnCalendar=hourly`, `Persistent=true` |
| `talaria-maintain.service` | `Type=oneshot` → `talaria maintain --gc-older-than 7d --quiet` |

`maintain` does:

1. **Status check** — drop registry entries whose directories were removed manually
2. **Age GC** — remove unpinned workspaces older than `--gc-older-than`
3. **Orphan report** — dirs under the workspace root not in the registry (not deleted)

Units live in `tools/systemd-services/`. See that directory’s README for linger notes.

You can run both the timer and `talariad`: the CLI prefers the daemon when up; the oneshot still works via daemon or local fallback.

## JSON registry

One file, human-readable, no database:

```json
{
  "version": 1,
  "next_note_id": 2,
  "workspaces": [
    {
      "id": "y4f8",
      "path": "/tmp/talaria/y4f8",
      "description": "Investigate mmap",
      "created_at": "2026-08-05T18:00:00Z",
      "last_used": "2026-08-05T18:30:00Z",
      "pinned": false,
      "tags": ["zig", "parser"],
      "notes": [
        {
          "id": 1,
          "body": "Need to benchmark mmap against buffered IO.",
          "created_at": "2026-08-05T18:05:00Z"
        }
      ]
    }
  ]
}
```

Writes are atomic (temp file + rename) and serialized by the exclusive lock.

## Design principles

1. **Local-first, daemon-optional** — same CLI either way; prefer daemon when up.
2. **Stdout = machine, stderr = human** — composable with the shell.
3. **Shell integration optional** — wrappers only for `cd`.
4. **Pinned vs disposable** — pin what you care about; `gc` the rest.
5. **One writer at a time** — flock + optional long-lived daemon.

## Development

```bash
make build
make test
make smoke
make smoke-daemon
make clean
```

CLI is built on [urfave/cli/v3](https://cli.urfave.org). Module path: `github.com/jmonroynieto/cliWorkflow_tk/talaria`.

| Binary | Package |
|--------|---------|
| `talaria` | `./cmd/talaria` |
| `talariad` | `./cmd/talariad` |
