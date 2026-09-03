package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
)

// configsyncCommand prints the adb invocation that moves the vault's
// configuration directory, and does not run it.
//
// Every other command in this tool exists because the operation needed
// judgment: what to merge, what a deletion means, which side is newer.
// Moving .obsidian/ needs none of that — it is a whole-directory overwrite
// in one direction, which adb already does correctly. What it does need is
// timing and intent, both of which belong to the person at the keyboard:
// Obsidian rewrites these files as it exits, so a copy made while the app
// is open can be overwritten seconds later by the app itself.
//
// So the command's job is to spare you assembling the paths, and to put the
// caveat next to the command rather than in a document you would have to
// remember to read.
var configsyncCommand = &cli.Command{
	Name:  "configsync",
	Usage: "print the adb command that copies " + obsidianDirName + "/ to or from the phone",
	Description: "Prints; does not copy. " + obsidianDirName + "/ is deliberately outside every\n" +
		"sync mode — Obsidian rewrites it constantly and its JSON does not\n" +
		"survive a union merge — so moving it is a deliberate act with a\n" +
		"whole-directory overwrite at the end of it. Read the command, close\n" +
		"Obsidian on the device, then run it yourself.",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		cfg, err := loadConfig(cmd.String("config"))
		if err != nil {
			return err
		}
		if err := cfg.validateForNotesync(); err != nil {
			return err
		}
		return printConfigsync(os.Stdout, cfg)
	},
}

func printConfigsync(w io.Writer, cfg *Config) error {
	local := filepath.Join(cfg.VaultLocalDir, obsidianDirName)
	remote := cfg.VaultRemoteDir + "/" + obsidianDirName

	// A serial only belongs on the command line when the config pins one;
	// adding it unasked would print a command that stops working the day
	// the device is replaced.
	var serial string
	if cfg.DeviceSerial != "" {
		serial = " -s " + cfg.DeviceSerial
	}

	fmt.Fprintf(w, "%s/ is not synced. To move it, close Obsidian on the phone, then:\n\n", obsidianDirName)
	// adb push of a directory copies its contents into the destination, so
	// the destination named here is the parent — naming the .obsidian path
	// itself would nest a second copy inside the existing one.
	fmt.Fprintf(w, "  this machine -> phone:\n    adb%s push %s %s\n\n", serial, shellArg(local), shellArg(cfg.VaultRemoteDir))
	fmt.Fprintf(w, "  phone -> this machine:\n    adb%s pull %s %s\n\n", serial, shellArg(remote), shellArg(cfg.VaultLocalDir))

	if _, err := os.Stat(local); err != nil {
		fmt.Fprintf(w, "note: %s does not exist here yet; only the pull direction makes sense\n\n", local)
	}
	fmt.Fprintf(w, "Both directions overwrite whole files, oldest to newest is not consulted,\n")
	fmt.Fprintf(w, "and neither side is backed up first. To take one file from the phone\n")
	fmt.Fprintf(w, "instead, `logSync sync --bubble <path>` does that with a diff shown first.\n")
	return nil
}

// shellArg quotes a path for the shell the user is about to paste into.
// Unquoted, a vault directory with a space in its name prints a command
// that silently copies the wrong thing.
func shellArg(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"\\$`*?[]()&;|<>#~") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
