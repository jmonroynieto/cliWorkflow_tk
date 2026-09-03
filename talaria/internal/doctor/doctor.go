// Package doctor diagnoses talaria runtime state: socket, lock, registry, workspace root.
package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/paths"
	"github.com/jmonroynieto/cliWorkflow_tk/talaria/internal/rpc"
)

// Status is the result of one check.
type Status string

const (
	StatusOK   Status = "ok"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
	StatusInfo Status = "info"
)

// Check is one diagnostic line.
type Check struct {
	Name   string
	Status Status
	Detail string
}

// Paths lists resolved filesystem locations.
type Paths struct {
	DataDir       string
	Registry      string
	Lock          string
	Socket        string
	PIDFile       string
	WorkspaceRoot string
	DurableRoot   string
}

// Report is the full doctor output.
type Report struct {
	Paths  Paths
	Checks []Check
	Hints  []string
}

// OK reports whether every check is ok or info (no fail/warn).
func (r Report) OK() bool {
	for _, c := range r.Checks {
		if c.Status == StatusFail || c.Status == StatusWarn {
			return false
		}
	}
	return true
}

// Healthy is true when no check failed (warns are allowed).
func (r Report) Healthy() bool {
	for _, c := range r.Checks {
		if c.Status == StatusFail {
			return false
		}
	}
	return true
}

// Run gathers diagnostics. It does not acquire the registry lock for long;
// lock probing uses a non-blocking try and releases immediately if acquired.
func Run(ctx context.Context) (Report, error) {
	var r Report

	dataDir, err := paths.DataDir()
	if err != nil {
		return r, err
	}
	reg, err := paths.RegistryPath()
	if err != nil {
		return r, err
	}
	lock, err := paths.LockPath()
	if err != nil {
		return r, err
	}
	sock, err := paths.SocketPath()
	if err != nil {
		return r, err
	}
	pidFile := paths.PIDFileBeside(sock)
	wsRoot := paths.WorkspaceRoot()
	durableRoot, _ := paths.DurableRoot()

	r.Paths = Paths{
		DataDir:       dataDir,
		Registry:      reg,
		Lock:          lock,
		Socket:        sock,
		PIDFile:       pidFile,
		WorkspaceRoot: wsRoot,
		DurableRoot:   durableRoot,
	}

	// --- socket ---
	r.Checks = append(r.Checks, checkSocket(ctx, sock))

	// --- lock + PID ---
	lockCheck, lockHints := checkLock(lock, pidFile, sock)
	r.Checks = append(r.Checks, lockCheck)
	r.Hints = append(r.Hints, lockHints...)

	// --- registry JSON ---
	r.Checks = append(r.Checks, checkRegistry(reg))

	// --- workspace roots writable ---
	r.Checks = append(r.Checks, checkWorkspaceRoot("workspace-root", wsRoot))
	if durableRoot != "" {
		dCheck := checkWorkspaceRoot("durable-root", durableRoot)
		if dCheck.Status == StatusOK && paths.IsVolatilePath(durableRoot) {
			dCheck.Status = StatusWarn
			dCheck.Detail += " — under volatile storage; reboot may wipe durable workspaces"
		}
		r.Checks = append(r.Checks, dCheck)
	}

	// --- pinned workspaces still on volatile (tmp) storage ---
	pinCheck, pinHints := checkPinnedVolatile(reg)
	r.Checks = append(r.Checks, pinCheck)
	r.Hints = append(r.Hints, pinHints...)

	// Stale daemon guidance when socket is down but lock/pid look live or leftover.
	r.Hints = append(r.Hints, staleDaemonHints(r)...)

	return r, nil
}

func checkSocket(ctx context.Context, sock string) Check {
	name := "socket"
	if _, err := os.Stat(sock); err != nil {
		if os.IsNotExist(err) {
			return Check{Name: name, Status: StatusInfo, Detail: fmt.Sprintf("not present (%s) — daemon not running", sock)}
		}
		return Check{Name: name, Status: StatusFail, Detail: fmt.Sprintf("stat %s: %v", sock, err)}
	}

	dialCtx, cancel := context.WithTimeout(ctx, 400*time.Millisecond)
	defer cancel()
	client, err := rpc.Dial(dialCtx, sock)
	if err != nil {
		return Check{
			Name:   name,
			Status: StatusFail,
			Detail: fmt.Sprintf("file exists but not reachable at %s: %v (stale socket?)", sock, err),
		}
	}
	defer client.Close()

	var ping rpc.PingResult
	if err := client.Call(dialCtx, rpc.MethodPing, nil, &ping); err != nil {
		return Check{Name: name, Status: StatusFail, Detail: fmt.Sprintf("ping failed: %v", err)}
	}
	ver := ping.Version
	if ver == "" {
		ver = "unknown"
	}
	return Check{Name: name, Status: StatusOK, Detail: fmt.Sprintf("reachable, ping ok (daemon %s)", ver)}
}

func checkLock(lockPath, pidFile, sock string) (Check, []string) {
	name := "lock"
	var hints []string

	// Ensure parent exists so OpenFile can create the lock if missing.
	_ = paths.EnsureDir(filepath.Dir(lockPath))

	held, err := probeLockHeld(lockPath)
	if err != nil {
		return Check{Name: name, Status: StatusFail, Detail: err.Error()}, hints
	}

	pidFromFile, pidFileLive, pidFileErr := readPIDFile(pidFile)
	holders := lockHolderPIDs(lockPath)

	var parts []string
	if held {
		parts = append(parts, "held")
	} else {
		parts = append(parts, "free")
	}

	if pidFromFile > 0 {
		if pidFileLive {
			parts = append(parts, fmt.Sprintf("pid file %d (alive)", pidFromFile))
		} else {
			parts = append(parts, fmt.Sprintf("pid file %d (dead process)", pidFromFile))
		}
	} else if pidFileErr == nil {
		// empty / missing handled below
	} else if !os.IsNotExist(pidFileErr) {
		parts = append(parts, fmt.Sprintf("pid file: %v", pidFileErr))
	}

	if len(holders) > 0 {
		parts = append(parts, fmt.Sprintf("fuser/lsof PIDs: %s", joinPIDs(holders)))
	}

	detail := strings.Join(parts, "; ")
	detail += fmt.Sprintf(" (%s)", lockPath)

	status := StatusOK
	suspect := false
	if held {
		// Held is normal when talariad is up.
		if pidFromFile > 0 && pidFileLive {
			status = StatusOK
			detail = fmt.Sprintf("held by PID %d (talariad pid file); %s", pidFromFile, lockPath)
			if len(holders) > 0 && !containsPID(holders, pidFromFile) {
				detail += fmt.Sprintf("; also fuser/lsof: %s", joinPIDs(holders))
				status = StatusWarn
				suspect = true
			}
		} else if len(holders) > 0 {
			status = StatusOK
			detail = fmt.Sprintf("held by PID(s) %s (%s)", joinPIDs(holders), lockPath)
		} else if pidFromFile > 0 && !pidFileLive {
			status = StatusWarn
			detail = fmt.Sprintf("held but pid file %d is dead — possible stale lock (%s)", pidFromFile, lockPath)
			suspect = true
		} else {
			// Held with no pid file: still ok (older daemon or local CLI), but show how to inspect.
			status = StatusOK
			detail = fmt.Sprintf("held (holder unknown — try fuser/lsof); %s", lockPath)
			suspect = true
		}
	} else if pidFromFile > 0 && pidFileLive {
		status = StatusWarn
		detail = fmt.Sprintf("free, but pid file claims live PID %d (inconsistent); %s", pidFromFile, lockPath)
		suspect = true
	} else if pidFromFile > 0 && !pidFileLive {
		status = StatusWarn
		detail = fmt.Sprintf("free; stale pid file %d (dead process); %s", pidFromFile, lockPath)
		suspect = true
	}

	if suspect {
		hints = append(hints, lockInspectHints(lockPath, pidFile, sock, pidFromFile)...)
	}

	return Check{Name: name, Status: status, Detail: detail}, hints
}

func checkRegistry(regPath string) Check {
	name := "registry"
	raw, err := os.ReadFile(regPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Check{
				Name:   name,
				Status: StatusInfo,
				Detail: fmt.Sprintf("not created yet (%s) — will appear on first use", regPath),
			}
		}
		return Check{Name: name, Status: StatusFail, Detail: fmt.Sprintf("read %s: %v", regPath, err)}
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return Check{Name: name, Status: StatusWarn, Detail: fmt.Sprintf("empty file (%s)", regPath)}
	}

	var doc struct {
		Version    int `json:"version"`
		Workspaces []struct {
			ID string `json:"id"`
		} `json:"workspaces"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Check{Name: name, Status: StatusFail, Detail: fmt.Sprintf("parse %s: %v", regPath, err)}
	}
	if doc.Version == 0 {
		doc.Version = 1
	}
	return Check{
		Name:   name,
		Status: StatusOK,
		Detail: fmt.Sprintf("parseable (version %d, %d workspace(s)); %s", doc.Version, len(doc.Workspaces), regPath),
	}
}

func checkWorkspaceRoot(name, root string) Check {
	if err := paths.EnsureDir(root); err != nil {
		return Check{Name: name, Status: StatusFail, Detail: fmt.Sprintf("mkdir %s: %v", root, err)}
	}
	// Probe writability with a temp file.
	f, err := os.CreateTemp(root, ".talaria-doctor-*")
	if err != nil {
		return Check{Name: name, Status: StatusFail, Detail: fmt.Sprintf("not writable (%s): %v", root, err)}
	}
	nameTmp := f.Name()
	_ = f.Close()
	_ = os.Remove(nameTmp)
	return Check{Name: name, Status: StatusOK, Detail: fmt.Sprintf("writable (%s)", root)}
}

func checkPinnedVolatile(regPath string) (Check, []string) {
	name := "pinned-volatile"
	raw, err := os.ReadFile(regPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Check{Name: name, Status: StatusOK, Detail: "no registry yet"}, nil
		}
		return Check{Name: name, Status: StatusFail, Detail: err.Error()}, nil
	}
	var doc struct {
		Workspaces []struct {
			ID     string `json:"id"`
			Path   string `json:"path"`
			Pinned bool   `json:"pinned"`
		} `json:"workspaces"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		// registry check already covers parse failures
		return Check{Name: name, Status: StatusInfo, Detail: "skipped (registry not parseable)"}, nil
	}
	var bad []string
	for _, ws := range doc.Workspaces {
		// Already under the configured durable root counts as "perdured"
		// even if that root happens to live under /tmp (tests / odd setups).
		if ws.Pinned && paths.IsVolatilePath(ws.Path) && !paths.IsDurablePath(ws.Path) {
			bad = append(bad, ws.ID)
		}
	}
	if len(bad) == 0 {
		return Check{Name: name, Status: StatusOK, Detail: "no pinned workspaces on volatile storage"}, nil
	}
	hints := []string{
		"pin keeps workspaces from gc; it does not survive reboot when the path is under /tmp.",
	}
	for _, id := range bad {
		hints = append(hints, fmt.Sprintf("  talaria perdure %s", id))
	}
	return Check{
		Name:   name,
		Status: StatusWarn,
		Detail: fmt.Sprintf("%d pinned workspace(s) under volatile path: %s", len(bad), strings.Join(bad, ", ")),
	}, hints
}

func readPIDFile(path string) (pid int, alive bool, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false, err
	}
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return 0, false, fmt.Errorf("empty pid file")
	}
	pid, err = strconv.Atoi(s)
	if err != nil {
		return 0, false, fmt.Errorf("invalid pid %q", s)
	}
	if pid <= 0 {
		return 0, false, fmt.Errorf("invalid pid %d", pid)
	}
	return pid, processAlive(pid), nil
}

func processAlive(pid int) bool {
	return signalZero(pid)
}

func lockInspectHints(lockPath, pidFile, sock string, pid int) []string {
	var h []string
	h = append(h, fmt.Sprintf("Inspect lock holders:  fuser -v %s", lockPath))
	h = append(h, fmt.Sprintf("                       lsof %s", lockPath))
	if pid > 0 {
		h = append(h, fmt.Sprintf("Kill stale talariad:    kill %d   # or: kill $(cat %s)", pid, pidFile))
	} else if _, err := os.Stat(pidFile); err == nil {
		h = append(h, fmt.Sprintf("Kill stale talariad:    kill $(cat %s)", pidFile))
	} else {
		h = append(h, "Kill stale talariad:    pkill talariad   # or kill the PID from fuser/lsof")
	}
	h = append(h, fmt.Sprintf("If the socket is orphaned after kill:  rm -f %s %s", sock, pidFile))
	return h
}

func staleDaemonHints(r Report) []string {
	// Only add a short summary if we already emitted lock hints or socket is stale.
	var socketFail, lockHeld bool
	for _, c := range r.Checks {
		if c.Name == "socket" && c.Status == StatusFail {
			socketFail = true
		}
		if c.Name == "lock" && strings.Contains(c.Detail, "held") {
			lockHeld = true
		}
	}
	if socketFail && lockHeld {
		return []string{
			"Socket unreachable while registry lock is held — likely a dead/stuck talariad. Use fuser/lsof on the .lock, kill that PID, then remove a stale socket if needed.",
		}
	}
	return nil
}

func lockHolderPIDs(lockPath string) []int {
	// Prefer lsof -t (quiet PIDs), then fuser.
	if pids := runPIDs("lsof", "-t", lockPath); len(pids) > 0 {
		return pids
	}
	// fuser prints "path:  pid1 pid2" on stderr typically.
	if pids := runFuser(lockPath); len(pids) > 0 {
		return pids
	}
	return nil
}

func runPIDs(name string, args ...string) []int {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil
	}
	cmd := exec.Command(path, args...)
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil
	}
	return parsePIDTokens(string(out))
}

func runFuser(lockPath string) []int {
	path, err := exec.LookPath("fuser")
	if err != nil {
		return nil
	}
	cmd := exec.Command(path, "-v", lockPath)
	out, _ := cmd.CombinedOutput()
	return parsePIDTokens(string(out))
}

func parsePIDTokens(s string) []int {
	seen := map[int]bool{}
	var out []int
	for _, f := range strings.Fields(s) {
		// strip trailing non-digits (fuser sometimes appends 'c' etc.)
		f = strings.TrimRight(f, "cmfrF")
		n, err := strconv.Atoi(f)
		if err != nil || n <= 0 {
			continue
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

func joinPIDs(pids []int) string {
	parts := make([]string, len(pids))
	for i, p := range pids {
		parts[i] = strconv.Itoa(p)
	}
	return strings.Join(parts, ", ")
}

func containsPID(pids []int, want int) bool {
	for _, p := range pids {
		if p == want {
			return true
		}
	}
	return false
}

// Format returns a human-readable report for the CLI.
func Format(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Paths\n")
	fmt.Fprintf(&b, "  data dir:        %s\n", r.Paths.DataDir)
	fmt.Fprintf(&b, "  registry:        %s\n", r.Paths.Registry)
	fmt.Fprintf(&b, "  lock:            %s\n", r.Paths.Lock)
	fmt.Fprintf(&b, "  socket:          %s\n", r.Paths.Socket)
	fmt.Fprintf(&b, "  pid file:        %s\n", r.Paths.PIDFile)
	fmt.Fprintf(&b, "  workspace root:  %s\n", r.Paths.WorkspaceRoot)
	fmt.Fprintf(&b, "  durable root:    %s\n", r.Paths.DurableRoot)
	fmt.Fprintf(&b, "\nChecks\n")
	for _, c := range r.Checks {
		fmt.Fprintf(&b, "  %-5s  %-14s  %s\n", "["+string(c.Status)+"]", c.Name, c.Detail)
	}
	if len(r.Hints) > 0 {
		fmt.Fprintf(&b, "\nHints\n")
		// Dedupe while preserving order.
		seen := map[string]bool{}
		for _, h := range r.Hints {
			if seen[h] {
				continue
			}
			seen[h] = true
			fmt.Fprintf(&b, "  %s\n", h)
		}
	}
	return b.String()
}
