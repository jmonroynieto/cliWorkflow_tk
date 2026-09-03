package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pydpll/errorutils"
	"github.com/urfave/cli/v3"
)

var (
	Version  string
	Revision = "0"
	CommitId string
)

var cmd = &cli.Command{
	Name:    "janus",
	Usage:   "SSH ProxyJump two-faced toggler — keep your tunnels consistent",
	Version: fmt.Sprintf("%s.%s (%s)", Version, Revision, CommitId),
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "file",
			Aliases: []string{"f"},
			Usage:   "Absolute path to SSH config file",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		cfg, err := loadJanusConfig()
		if err != nil {
			return err
		}

		sshPath := cmd.String("file")
		if sshPath == "" {
			sshPath = cfg.SSHConfigPath
		}
		if sshPath != "" && !filepath.IsAbs(sshPath) {
			return fmt.Errorf("SSH config path must be absolute")
		}

		args := cmd.Args().Slice()

		// Check for report
		if cmd.Bool("report") || (len(args) > 0 && strings.Contains(args[0], "report")) {
			return showReport(sshPath)
		}

		if len(args) == 0 {
			return performToggle(sshPath, cfg.BackupDir, "", true)
		}

		if len(args) == 1 {
			switch strings.ToLower(args[0]) {
			case "wsl":
				return performToggle(sshPath, cfg.BackupDir, StateWSL, false)
			case "jump-gate", "jumpgate":
				return performToggle(sshPath, cfg.BackupDir, StateJumpGate, false)
			}
		}

		return cli.ShowAppHelp(cmd)
	},
	Commands: []*cli.Command{
		{
			Name:    "report",
			Aliases: []string{"-report"},
			Usage:   "The two-faced glance — survey current proxy state per block",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				cfg, _ := loadJanusConfig()
				sshPath := cmd.String("file")
				if sshPath == "" {
					sshPath = cfg.SSHConfigPath
				}
				return showReport(sshPath)
			},
		},
	},
}

const (
	defaultSSHConfig    = ".ssh/config"
	janusConfigDir      = ".config/janus"
	janusConfigFile     = "config"
	defaultBackupSubdir = "backups"
)

var (
	proxyWslRe  = regexp.MustCompile(`(?i)^\s*(#?\s*)ProxyJump\s+wsl\s*$`)
	proxyJumpRe = regexp.MustCompile(`(?i)^\s*(#?\s*)Proxyjump\s+jump-gate\s*$`)
	hostRe      = regexp.MustCompile(`(?i)^\s*Host\s+`)
)

func main() {
	err := cmd.Run(context.Background(), os.Args)
	errorutils.ExitOnFail(err)
}

// Config holds runtime configuration
type Config struct {
	SSHConfigPath string
	BackupDir     string
}

// ProxyState represents which proxy is active
type ProxyState string

const (
	StateWSL      ProxyState = "wsl"
	StateJumpGate ProxyState = "jump-gate"
	StateUnknown  ProxyState = "unknown"
)

// Block represents a toggleable host block
type Block struct {
	StartLine    int
	EndLine      int
	WSLLine      int // line index of ProxyJump wsl (or -1)
	JumpGateLine int // line index of Proxyjump jump-gate (or -1)
	CurrentState ProxyState
}

// loadJanusConfig reads or creates the janus configuration file.
// Only accepts absolute paths for backup_dir.
func loadJanusConfig() (*Config, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}

	home := usr.HomeDir
	janusDir := filepath.Join(home, janusConfigDir)
	janusCfgPath := filepath.Join(janusDir, janusConfigFile)

	// Ensure janus config directory exists
	if err := os.MkdirAll(janusDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create janus config dir: %w", err)
	}

	cfg := &Config{
		SSHConfigPath: filepath.Join(home, defaultSSHConfig),
	}

	// Try to read existing config
	data, err := os.ReadFile(janusCfgPath)
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "backup_dir=") {
				dir := strings.TrimPrefix(line, "backup_dir=")
				dir = strings.TrimSpace(dir)
				if filepath.IsAbs(dir) {
					cfg.BackupDir = dir
				}
			}
		}
	}

	// Set sensible default if not configured
	if cfg.BackupDir == "" {
		cfg.BackupDir = filepath.Join(home, ".ssh", defaultBackupSubdir)
		content := fmt.Sprintf("backup_dir=%s\n", cfg.BackupDir)
		if err := os.WriteFile(janusCfgPath, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("failed to write default janus config: %w", err)
		}
		fmt.Printf("Created default configuration at %s\n", janusCfgPath)
		fmt.Printf("Backup directory set to: %s\n\n", cfg.BackupDir)
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(cfg.BackupDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory %s: %w", cfg.BackupDir, err)
	}

	return cfg, nil
}

// parseBlocks finds all toggleable blocks containing both proxy options
func parseBlocks(lines []string) []Block {
	var blocks []Block
	var current *Block

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if hostRe.MatchString(trimmed) {
			// Save previous valid block
			if current != nil && current.WSLLine != -1 && current.JumpGateLine != -1 {
				blocks = append(blocks, *current)
			}
			current = &Block{
				StartLine:    i,
				WSLLine:      -1,
				JumpGateLine: -1,
			}
			continue
		}

		if current == nil {
			continue
		}

		if proxyWslRe.MatchString(line) {
			current.WSLLine = i
			if !strings.HasPrefix(strings.TrimSpace(line), "#") {
				current.CurrentState = StateWSL
			}
		}
		if proxyJumpRe.MatchString(line) {
			current.JumpGateLine = i
			if !strings.HasPrefix(strings.TrimSpace(line), "#") {
				current.CurrentState = StateJumpGate
			}
		}
	}

	// Add last block if valid
	if current != nil && current.WSLLine != -1 && current.JumpGateLine != -1 {
		current.EndLine = len(lines) - 1
		blocks = append(blocks, *current)
	}

	return blocks
}

// toggleLine comments or uncomments a proxy line
func toggleLine(line string, makeActive bool) string {
	trimmed := strings.TrimSpace(line)
	isCommented := strings.HasPrefix(trimmed, "#")

	if makeActive {
		if isCommented {
			// Remove the leading comment
			return strings.TrimPrefix(trimmed, "#")
		}
		return line
	}
	// Make inactive (comment it)
	if !isCommented {
		return "# " + trimmed
	}
	return line
}

// applyStateToBlock updates the two proxy lines in a block
func applyStateToBlock(lines []string, block Block, target ProxyState) []string {
	newLines := make([]string, len(lines))
	copy(newLines, lines)

	if block.WSLLine == -1 || block.JumpGateLine == -1 {
		return newLines
	}

	makeWSLActive := target == StateWSL
	makeJumpActive := target == StateJumpGate

	newLines[block.WSLLine] = toggleLine(lines[block.WSLLine], makeWSLActive)
	newLines[block.JumpGateLine] = toggleLine(lines[block.JumpGateLine], makeJumpActive)

	return newLines
}

// writeFileAtomic writes via temp file + rename so an active ssh process
// never catches the config half-written.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".janus-tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to sync temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file %s: %w", tmpPath, err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("failed to chmod temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename %s to %s: %w", tmpPath, path, err)
	}
	return nil
}

// createBackup writes a timestamped backup of srcData — the exact bytes we
// already have in hand, not a fresh re-read off disk.
func createBackup(srcData []byte, backupDir string) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("config.bak.%s", timestamp)
	backupPath := filepath.Join(backupDir, backupName)

	if err := writeFileAtomic(backupPath, srcData, 0o644); err != nil {
		return "", err
	}

	return backupPath, nil
}

// performToggle is the core logic for both toggle and explicit set
func performToggle(sshPath, backupDir string, targetState ProxyState, isToggle bool) error {
	info, err := os.Stat(sshPath)
	if err != nil {
		return fmt.Errorf("failed to stat SSH config at %s: %w", sshPath, err)
	}

	data, err := os.ReadFile(sshPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH config at %s: %w", sshPath, err)
	}
	lines := strings.Split(string(data), "\n")

	blocks := parseBlocks(lines)
	if len(blocks) == 0 {
		fmt.Println("No toggleable blocks found (Host blocks containing both 'ProxyJump wsl' and 'Proxyjump jump-gate').")
		return nil
	}

	var finalState ProxyState

	if isToggle {
		first := blocks[0]
		switch first.CurrentState {
		case StateWSL:
			finalState = StateJumpGate
		case StateJumpGate:
			finalState = StateWSL
		default:
			finalState = StateWSL
		}
		fmt.Printf("First block is currently using: %s\n", first.CurrentState)
		fmt.Printf("→ Switching ALL blocks to: %s\n\n", finalState)
	} else {
		finalState = targetState
		fmt.Printf("Forcing ALL blocks to: %s\n\n", finalState)
	}

	// Apply changes to every matching block
	modifiedLines := lines
	for _, block := range blocks {
		modifiedLines = applyStateToBlock(modifiedLines, block, finalState)
	}

	// Backup first
	backupPath, err := createBackup(data, backupDir)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	fmt.Printf("Backup created: %s\n", backupPath)

	// Write the new config, keeping the original file permissions
	newData := []byte(strings.Join(modifiedLines, "\n"))
	if err := writeFileAtomic(sshPath, newData, info.Mode().Perm()); err != nil {
		return fmt.Errorf("failed to write updated SSH config: %w", err)
	}

	fmt.Println("Success. All relevant ProxyJump rules are now consistent.")
	return nil
}

// showReport prints a nice status table
func showReport(sshPath string) error {
	data, err := os.ReadFile(sshPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH config at %s: %w", sshPath, err)
	}
	lines := strings.Split(string(data), "\n")

	blocks := parseBlocks(lines)
	if len(blocks) == 0 {
		fmt.Println("No toggleable proxy blocks found.")
		return nil
	}

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  JANUS REPORT — The Two-Faced Glance                                        ║")
	fmt.Println("╠════════════════════════════════════════════════════════════════════════════╣")

	for i, block := range blocks {
		hostLine := strings.TrimSpace(lines[block.StartLine])
		hostName := strings.TrimPrefix(hostLine, "Host ")
		hostName = strings.TrimPrefix(hostName, "host ")
		hostName = strings.TrimSpace(hostName)

		wslActive := false
		jumpActive := false

		if block.WSLLine != -1 {
			wslActive = !strings.HasPrefix(strings.TrimSpace(lines[block.WSLLine]), "#")
		}
		if block.JumpGateLine != -1 {
			jumpActive = !strings.HasPrefix(strings.TrimSpace(lines[block.JumpGateLine]), "#")
		}

		wslStatus := "✗ inactive"
		if wslActive {
			wslStatus = "✓ ACTIVE"
		}
		jumpStatus := "✗ inactive"
		if jumpActive {
			jumpStatus = "✓ ACTIVE"
		}

		active := "none"
		if wslActive && jumpActive {
			active = "BOTH (inconsistent!)"
		} else if wslActive {
			active = "wsl"
		} else if jumpActive {
			active = "jump-gate"
		}

		fmt.Printf("║  %2d. %-28s  wsl: %-12s   jump-gate: %-12s   → %s\n",
			i+1, hostName, wslStatus, jumpStatus, active)
	}

	fmt.Println("╚════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println("Legend: ✓ = currently active (uncommented)   |   ✗ = commented out")
	fmt.Println()
	return nil
}
