# Dokwerker (DockerHelper CLI)

A streamlined Go CLI for managing containerized development environments across Go and TypeScript stacks. It automates boilerplate setup, handles container footprints, and maps host `UID`/`GID` to prevent file permission issues with bind mounts.

## Quick Start & Examples

```bash
# 1. Full workflow: Initialize, build, and jump into a TypeScript dev shell with a custom project name
dokwerker fresh --stack ts --project bibtex-1

# 2. Just copy Go boilerplate files into the current directory
dokwerker init --stack go

# 3. Build and launch your Go containers in the background (default: bridge network)
dokwerker up --stack go

# 3b. Same, but opt into host networking for this run only (Linux; does not edit docker-compose.yml)
dokwerker up --stack go --host-net
dokwerker fresh -s ts -p bibtex-1 --host-net

# 4. Open an interactive shell (auto-starts if down; recovers network on failure)
dokwerker shell --stack ts --project bibtex-1
dokwerker shell -s go --host-net   # auto-up with host net if the service is down

# 5. Inspect services, run state, network mode, and attached networks
dokwerker status
dokwerker status --project bibtex-1 --stack ts

# 6. Fast network recovery after sleep / VPN / daemon glitches (no rebuild)
dokwerker netfix                         # all running containers
dokwerker netfix --project bibtex-1      # one compose project
dokwerker netfix --service-only -s go    # only the stack's mapped service

# 7. Stop and clean up the project stack
dokwerker down --project bibtex-1
```

## Commands

| Command | Aliases | What it does |
|---------|---------|--------------|
| `init` | | Copy stack boilerplate (`docker-compose.yml` + `Dockerfile`) into cwd |
| `up` | | `docker compose up -d --build` |
| `shell` | | Ensure service is running, then `compose exec -it … bash`. On probe failure: **netfix that service** (bridge) or **force-recreate** (`--host-net`) |
| `status` | `ps`, `st` | Table of services: state, status, container id, **net mode**, networks; marks the `--stack` shell target |
| `netfix` | `reconnect`, `fixnet` | Force disconnect+reconnect bridge networks (parallel). Skips host-network containers. `--service-only` / `-S` limits to the stack service |
| `down` | | `docker compose down` |
| `fresh` | `reset` | `init` (if needed) → `up` → `shell` |

### Global flags

| Flag | Default | Meaning |
|------|---------|---------|
| `-s` / `--stack` | `go` | Stack → service map: `go` → `go-dev`, `ts` → `obsidian-dev` |
| `-p` / `--project` | *(dir name)* | Docker Compose **project name** — labels this stack instance so you can run several in parallel (`bibtex-1`, …) |
| `-H` / `--host-net` | **off** | Opt-in host networking for the stack service (see below) |

## Host networking (`--host-net` / `-H`)

**Default stays secure/isolated:** vendored and generated `docker-compose.yml` files keep Docker’s **bridge** network. Host mode is never written into those files by `init`.

When you pass `--host-net`, dokwerker:

1. Writes a **temporary** compose override (under your OS temp dir), e.g.  
   `services: { go-dev: { network_mode: host } }`
2. Runs compose as  
   `docker compose -f docker-compose.yml -f <temp-override> …`
3. Leaves your project `docker-compose.yml` **unchanged**

```bash
# Create/recreate with host stack (Linux Docker Engine)
dokwerker up -s go -p myproj --host-net
dokwerker fresh -s ts -p bibtex-1 -H

# Exec only: if the container is already running, exec works either way;
# pass --host-net so auto-start / recovery also uses host mode
dokwerker shell -s go -p myproj --host-net
```

| | Bridge (default) | `--host-net` |
|--|------------------|--------------|
| Isolation | Project bridge network | Shares host netns (no bridge bubble) |
| Localhost → host services | Needs extra wiring | Same as host |
| Sleep/VPN stale endpoints | Use `netfix` / shell recovery | Usually not an issue |
| `ports:` mappings | Work | Ignored (already on host) |
| Platform | All | **True host net on Linux**; not the same on Docker Desktop Mac/Windows |

**Switching modes:** network mode is fixed at container create time. To move bridge → host (or back):

```bash
dokwerker down -p myproj
dokwerker up -s go -p myproj --host-net   # or omit --host-net for bridge
```

## Why `shell` stopped needing `reset`

Previously `shell` was a thin `compose exec`. If the container was stopped or its network endpoint was stale (sleep/VPN), exec failed and the workaround was a full `fresh`/`reset` rebuild.

Now `shell`:

1. Starts the stack if the service is not running (honors `--host-net` on that auto-up)
2. Probes with `exec -T … true`
3. On probe failure:
   - **bridge:** reconnects **only that service’s** networks
   - **`--host-net`:** force-recreates the service (bridge reconnect does not apply)
4. Opens an interactive TTY shell

Use `fresh` only when you actually need a rebuild/re-init.

## Network fix (`netfix`)

```bash
dokwerker netfix                 # all running containers
dokwerker netfix -p myproj       # one compose project
dokwerker netfix -S -s go         # only go-dev (or ts → obsidian-dev)
```

This is the Go equivalent of (and faster than) the serial bash loop:

```bash
for c in $(docker ps -q); do
  for n in $(docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}' "$c"); do
    docker network disconnect -f "$n" "$c" && docker network connect "$n" "$c"
  done
done
```

Differences vs the shell one-liner:

- **`disconnect -f`** — force-clears half-dead endpoints
- **Parallel** — up to 8 containers at a time
- **Optional project / service scope** via `-p` and `--service-only`
- **Skips `network_mode: host` containers** (nothing to reconnect)
- **`shell` auto-calls service-scoped netfix** once if `exec` fails (bridge mode)

If you still want a shell function (no binary):

```bash
dnetfix() {
  docker ps -q | xargs -r -P8 -I{} sh -c '
    for n in $(docker inspect -f "{{range \$k,\$v := .NetworkSettings.Networks}}{{\$k}} {{end}}" "$1"); do
      docker network disconnect -f "$n" "$1" && docker network connect "$n" "$1"
    done
  ' _ {}
}
```

## Status

```bash
dokwerker status -p bibtex-1 -s ts
```

Example columns:

```
SERVICE        STATE    STATUS         CONTAINER    NET MODE  NETWORKS              NOTE
obsidian-dev   running  Up 2 hours     a1b2c3d4e5f6 bridge    bibtex-1_default      ← shell target
go-dev         running  Up 10 minutes  f6e5d4c3b2a1 host      (host stack)          ← shell target
```

## Environment

- `scriptLoc` — required for `init` / auto-init in `fresh`; points at the tree that contains `boilerplate/docker/…` (your `docker-vendored` files can live there)
- Host `UID`/`GID` are injected into compose so bind mounts stay writable

## Build

```bash
go build -o dokwerker .
```
