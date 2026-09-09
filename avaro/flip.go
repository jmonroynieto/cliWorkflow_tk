package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

var flipCommand = &cli.Command{
	Name:  "flip",
	Usage: "toggle conservation mode, escalating with sudo when needed",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		path, err := conservationPath()
		if err != nil {
			return err
		}
		before, err := readConservation(path)
		if err != nil {
			return err
		}
		target := !before

		if err := writeConservation(path, target); err != nil {
			return err
		}

		// Read back rather than trusting the write. A mismatch is reported rather than
		// raised: the controller may simply not have reflected the store yet, and
		// failing a write that did land is worse than a note about one that did not.
		after, err := readConservation(path)
		if err != nil {
			return err
		}
		if after != target {
			fmt.Printf("conservation mode: still %s  (%s)\n", stateWord(after), stateMeaning(after))
			fmt.Printf("\nwrote %s to %s but it reads back %s. Either the controller has not\n", stateWord(target), path, stateWord(after))
			fmt.Printf("reflected the store yet or it refused the transition; re-run avaro to see which.\n")
			return nil
		}

		fmt.Printf("conservation mode: %s -> %s  (%s)\n", stateWord(before), stateWord(after), stateMeaning(after))
		warnTLPWillRevert(after)
		return nil
	},
}

// warnTLPWillRevert says so when the flip disagrees with TLP's configuration, and spells
// out which events actually reassert it. The disagreement is not necessarily a mistake:
// it is how a temporary uncapping is expressed.
func warnTLPWillRevert(now bool) {
	setting := readTLPSetting()
	if !setting.Found {
		return
	}
	configured, recognised := setting.Enabled()
	if !recognised || configured == now {
		return
	}

	fmt.Printf("\nthis will not stick: %s=%s in %s asks for %s.\n", tlpStopKey, setting.Value, setting.Source, stateWord(configured))
	fmt.Print(reassertNote(configured, now))
	fmt.Printf("To make %s permanent instead, set %s=%s in %s and run: sudo tlp start\n", stateWord(now), tlpStopKey, tlpValueFor(now), setting.Source)
}

func tlpValueFor(on bool) string {
	if on {
		return "1"
	}
	return "0"
}
