# gromula

**gromula** — because your PATH deserves to be managed, not just mutated.

- Avoids duplicate entries
- Tracks **where** each path was added from (via `--source` or automatic shell wrappers)
- Persists state so you can inspect how your PATH was built during a session
- Defaults to **append** (safer) with explicit `--prepend` / `-p` when you need highest priority
- Works beautifully as a drop-in replacement in your shell functions

## Installation

Or build from source:

```bash
git clone https://github.com/jmonroynieto/cliWorkflow_tk
make
make install
```

## Recommended Ergonomic Wrappers

Add this to your `~/.bashrc`, `~/.zshrc`, or a shared `~/.config/shell/aliases.sh`:

```bash
# ============================================
# gromula - safe PATH management with tracking
# ============================================

g_add() {
    local src="${BASH_SOURCE[1]:-$0}"
    export PATH="$(gromula add "$@" --source "$src")"
}

g_p() {
    local src="${BASH_SOURCE[1]:-$0}"
    export PATH="$(gromula add "$@" --source "$src" --prepend)"
}

g_rm() {
    local src="${BASH_SOURCE[1]:-$0}"
    export PATH="$(gromula remove "$@" --source "$src")"
}

### Usage examples

```bash
# Append (default, recommended for most additions)
gromula_add "$HOME/.local/bin"
gromula_add "/opt/fungal-analysis/bin" "/opt/maldi-tools/bin"

# Prepend when you want to override system tools
gromula add "$HOME/projects/devbin" -p

# Remove
gromula remove "/opt/old-conda/bin"

```

## Core Commands

| Command          | Description                                      | Default behavior      |
|------------------|--------------------------------------------------|-----------------------|
| `gromula add`    | Add path(s)                                      | append                |
| `gromula remove` | Remove path(s)                                   | -                     |
| `gromula clean`  | Deduplicate current `$PATH` (no state change)    | -                     |
| `gromula path`   | Print clean PATH string only (bash-ready for export) | -                 |
| `gromula show`   | Pretty print tracked entries + provenance        | -                     |
| `gromula history`| Show location of the operations audit log        | -                     |
| `gromula init`   | Initialize state + optional new session          | -                     |

### Flags (common)

- `--source`, `-s` — identification of the caller (script, function, etc.)
- `--prepend`, `-p` — force the path(s) to the front of PATH
- `--session-id` — associate with a specific shell session (also reads `$GROMULA_SESSION_ID`)

## How State & Tracking Work

- State lives in `$XDG_STATE_HOME/gromula/` (falls back to `~/.local/state/gromula/`)
- `pathstate.json` — current ordered list of entries with full metadata
- `operations.jsonl` — append-only audit log of every add/remove

You can safely delete the directory to reset tracking (your real `$PATH` is never touched by `reset`).

## Legacy / One-liner Compatibility

The original bare syntax still works:

```bash
export PATH="$(gromula /new/path -/old/path)"
```

This mode always prepends new items (for compatibility with the very first prototype).

## Philosophy

`gromula` exists because:

- PATH pollution from conda, modules, cargo, go, mise, etc. is painful
- You want to know **which** of your many shell snippets added a particular directory
- Ordering sometimes matters, so we made the safe choice the default (append) and made prepending explicit

## Future Ideas

- TUI `reorder` / `history` with bubbletea (when needed)
- Integration with `errorutils` from pydpll for richer error context

