package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

var notesCommand = &cli.Command{
	Name:  "notes",
	Usage: "print the commands that make the current setting permanent",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		// Point at the file TLP actually reads the assignment from, so the printed
		// edit lands where it takes effect rather than in a shadowed drop-in.
		target := tlpMainConf
		if setting := readTLPSetting(); setting.Found {
			target = setting.Source
		}

		fmt.Printf("#%s=0 off/1 on\n", tlpStopKey)
		fmt.Printf("sudo vim %s\n", target)
		fmt.Printf("sudo tlp start\n")
		return nil
	},
}
