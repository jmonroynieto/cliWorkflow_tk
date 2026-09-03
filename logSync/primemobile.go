package main

import (
	"context"
	"fmt"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/adbx"
	"github.com/urfave/cli/v3"
)

var primeMobileCommand = &cli.Command{
	Name:  "primeMobile",
	Usage: "create the configured remote directories on the device",
	Description: "Creates remote_dir and vault_remote_dir on the phone if they are\n" +
		"missing, which is what doctor asks for when it reports a directory\n" +
		"as absent. Nothing is created on this machine: the local paths are\n" +
		"your own tree, and creating one would turn a typo in the config into\n" +
		"a setup that looks fine and syncs into the wrong place.",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		cfg, err := loadConfig(cmd.String("config"))
		if err != nil {
			return err
		}
		dirs := remoteDirChecks(cfg)
		if len(dirs) == 0 {
			return fmt.Errorf("no remote directories configured; set remote_dir or vault_remote_dir first")
		}
		results, err := adbx.Prime(adbx.DoctorConfig{
			DeviceSerial: cfg.DeviceSerial,
			RemoteDirs:   dirs,
		})
		for _, r := range results {
			state := "already present"
			if r.Created {
				state = "created"
			}
			fmt.Printf("%-16s %s  (%s)\n", r.Key, state, r.Path)
		}
		return err
	},
}

// remoteDirChecks is the set of device directories both doctor and
// primeMobile work from, so the paths one reports missing are the paths the
// other creates.
func remoteDirChecks(cfg *Config) []adbx.RemoteDir {
	var dirs []adbx.RemoteDir
	if cfg.RemoteDir != "" {
		dirs = append(dirs, adbx.RemoteDir{Key: "remote_dir", Path: cfg.RemoteDir})
	}
	if cfg.VaultRemoteDir != "" {
		dirs = append(dirs, adbx.RemoteDir{Key: "vault_remote_dir", Path: cfg.VaultRemoteDir})
	}
	return dirs
}
