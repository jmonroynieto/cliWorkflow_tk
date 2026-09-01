package main

import (
	"context"
	"fmt"

	"github.com/pydpll/errorutils"
	"github.com/urfave/cli/v3"
)

func statusAction(ctx context.Context, cmd *cli.Command) error {
	// The root command carries both an Action and a Commands list, so a mistyped
	// subcommand arrives here as a stray argument instead of an unknown-command error.
	if args := cmd.Args().Slice(); len(args) > 0 {
		return errorutils.NewReport(
			fmt.Sprintf("unknown command %q; avaro takes no arguments, see avaro --help", args[0]),
			"3nQvKp7bLdT",
			errorutils.WithExitCode(1),
		)
	}

	path, err := conservationPath()
	if err != nil {
		return err
	}
	live, err := readConservation(path)
	if err != nil {
		return err
	}

	fmt.Printf("conservation mode: %-3s  (%s)\n", stateWord(live), stateMeaning(live))
	fmt.Printf("sysfs:             %s\n", path)
	reportTLP(live)
	return nil
}

// reportTLP prints the configured state next to the live one. TLP reapplies its
// configuration on every AC/battery transition, so a divergence here means the live
// value has a short shelf life.
func reportTLP(live bool) {
	setting := readTLPSetting()
	if !setting.Found {
		fmt.Printf("tlp config:        no %s assignment; nothing will reassert this\n", tlpStopKey)
		return
	}

	configured, recognised := setting.Enabled()
	if !recognised {
		fmt.Printf("tlp config:        %s=%s in %s, unrecognised (expected 0 or 1)\n", tlpStopKey, setting.Value, setting.Source)
		return
	}

	fmt.Printf("tlp config:        %s=%s in %s -> %s\n", tlpStopKey, setting.Value, setting.Source, stateWord(configured))
	if configured != live {
		fmt.Printf("\ndivergence: TLP is configured for %s but the controller is %s.\n", stateWord(configured), stateWord(live))
		fmt.Printf("TLP reapplies its configuration on the next AC or battery transition, which will\n")
		fmt.Printf("put this back to %s. Edit %s to make the current state stick.\n", stateWord(configured), setting.Source)
	}
}
