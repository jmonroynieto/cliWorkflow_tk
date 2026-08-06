package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DataDir returns the XDG data directory for talaria state
// (JSON registry + socket), e.g. ~/.local/share/talaria.
func DataDir() (string, error) {
	if d := os.Getenv("TALARIA_DATA_DIR"); d != "" {
		return d, nil
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "talaria"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", "talaria"), nil
}

// WorkspaceRoot returns the directory that holds disposable workspace
// folders, default /tmp/talaria. Override with TALARIA_WORKSPACE_ROOT.
// These trees do not survive reboot when under tmp.
func WorkspaceRoot() string {
	if d := os.Getenv("TALARIA_WORKSPACE_ROOT"); d != "" {
		return d
	}
	return filepath.Join(os.TempDir(), "talaria")
}

// DurableRoot returns the directory for reboot-safe workspaces.
// Default: <DataDir>/workspaces (e.g. ~/.local/share/talaria/workspaces).
// Override with TALARIA_DURABLE_ROOT.
func DurableRoot() (string, error) {
	if d := os.Getenv("TALARIA_DURABLE_ROOT"); d != "" {
		return d, nil
	}
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "workspaces"), nil
}

// UnderRoot reports whether path is the same as root or a subdirectory of it.
func UnderRoot(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// IsDurablePath reports whether path lives under the configured durable root.
func IsDurablePath(path string) bool {
	root, err := DurableRoot()
	if err != nil {
		return false
	}
	return UnderRoot(path, root)
}

// IsVolatilePath reports whether path is likely wiped on reboot
// (under the process temp dir, /tmp, or /var/tmp).
func IsVolatilePath(path string) bool {
	if path == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	candidates := []string{
		os.TempDir(),
		"/tmp",
		"/var/tmp",
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if UnderRoot(abs, c) {
			return true
		}
	}
	return false
}

// RegistryPath returns the full path to the workspaces JSON registry.
func RegistryPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "workspaces.json"), nil
}

// LockPath returns the sidecar flock file for the registry
// (workspaces.json.lock next to the registry).
func LockPath() (string, error) {
	reg, err := RegistryPath()
	if err != nil {
		return "", err
	}
	return reg + ".lock", nil
}

// SocketPath returns the Unix domain socket used by talariad.
// Override with TALARIA_SOCKET.
func SocketPath() (string, error) {
	if s := os.Getenv("TALARIA_SOCKET"); s != "" {
		return s, nil
	}
	// Prefer XDG_RUNTIME_DIR when available (per-user, cleaned on logout).
	if runtime := os.Getenv("XDG_RUNTIME_DIR"); runtime != "" {
		return filepath.Join(runtime, "talaria.sock"), nil
	}
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "talaria.sock"), nil
}

// PIDFilePath returns the path of the daemon PID file (next to the socket).
// talaria.sock → talaria.pid in the same directory.
func PIDFilePath() (string, error) {
	sock, err := SocketPath()
	if err != nil {
		return "", err
	}
	return PIDFileBeside(sock), nil
}

// PIDFileBeside derives the PID file path from a socket path.
func PIDFileBeside(socketPath string) string {
	dir := filepath.Dir(socketPath)
	base := filepath.Base(socketPath)
	stem := strings.TrimSuffix(base, ".sock")
	if stem == base {
		// Custom name without .sock suffix.
		return socketPath + ".pid"
	}
	return filepath.Join(dir, stem+".pid")
}

// EnsureDir creates dir if it does not exist.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}
