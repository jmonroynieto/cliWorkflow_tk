# systemd user units for talaria

Wake-on-call maintenance: a **timer** fires an **oneshot** service that runs
`talaria maintain` — no long-running process required for GC/status checks.

These files are **assets** shipped with the module. Prefer installing them from
your **toolkit / `go.work` root Makefile**, not as part of building the
`talaria` binary itself.

| Unit | Role |
|------|------|
| `talaria-maintain.timer` | On boot (~1 min) + hourly; `Persistent=true` |
| `talaria-maintain.service` | `Type=oneshot` → `talaria maintain --gc-older-than 7d --quiet` |

## What `maintain` does

1. **Status check** — drop registry entries whose directories were deleted manually  
   (`rm -rf /tmp/talaria/xxxx` outside talaria).
2. **Age GC** — remove unpinned workspaces older than `--gc-older-than` (default 7d).
3. **Orphan scan** — report dirs under the workspace root not in the registry  
   (informational only; not deleted).

Interactive CLI / optional `talariad` stay as they are. This timer is the
scheduled path.

## Install from the toolkit (`go.work` root)

Binaries and units are two different concerns:

| Step | Where | What |
|------|--------|------|
| Build / install `talaria` + `talariad` | toolkit `make install` | copy into `install_dir` (e.g. `~/Local/bin`) |
| Install systemd units | toolkit `make install-systemd` (or similar) | copy unit files into `~/.config/systemd/user` |

Example toolkit Makefile targets (paths relative to the workspace root that
contains `talaria/` via `go work use`):

```makefile
install_dir := /home/pollo/Local/bin/
UNITDIR     ?= $(HOME)/.config/systemd/user

# After the normal TOOLS install loop has placed talaria on install_dir:

.PHONY: install-systemd
install-systemd:
	install -d $(UNITDIR)
	install -m 644 talaria/tools/systemd-services/talaria-maintain.service $(UNITDIR)/
	install -m 644 talaria/tools/systemd-services/talaria-maintain.timer $(UNITDIR)/
	@echo "Installed units to $(UNITDIR)"
	@echo "  systemctl --user daemon-reload"
	@echo "  systemctl --user enable --now talaria-maintain.timer"
```

Then once per machine:

```bash
make install                 # binaries → install_dir
make install-systemd         # units → systemd --user
systemctl --user daemon-reload
systemctl --user enable --now talaria-maintain.timer
```

### PATH inside the unit

`systemd --user` does **not** inherit your interactive shell `PATH`. The
service sets:

```
Environment=PATH=%h/Local/bin:%h/.local/bin:/usr/local/bin:/usr/bin:/bin
ExecStart=talaria maintain --gc-older-than 7d --quiet
```

So `talaria` is resolved by name. If your `install_dir` is neither `~/Local/bin`
nor `~/.local/bin`, either:

1. Add that directory to the unit’s `Environment=PATH=…` when you install
   (e.g. `sed` / `envsubst` in the toolkit target), or  
2. Point `ExecStart` at the absolute binary:
   `ExecStart=$(install_dir)/talaria maintain …`

Do that substitution in the **toolkit** install step; keep the unit files here
as the portable default.

### Inspect

```bash
systemctl --user list-timers 'talaria-*'
systemctl --user status talaria-maintain.timer
journalctl --user -u talaria-maintain.service -n 20
systemctl --user start talaria-maintain.service   # run once now
```

## Linger (optional)

User timers only fire while logged in unless lingering is enabled:

```bash
loginctl enable-linger "$USER"
```

## Relationship to `talariad`

| Component | When to use |
|-----------|-------------|
| `talaria-maintain.timer` | Scheduled reconcile + GC (recommended default) |
| `talariad` | Long-lived socket for multi-client access; optional in-process maintain ticker |

You can run both: the CLI prefers the daemon when up; the oneshot still works
via daemon or local fallback.
