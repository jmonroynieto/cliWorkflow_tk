package main

import (
	"context"
	"fmt"

	"github.com/jmonroynieto/cliWorkflow_tk/fickleFinger/idGen"
	"github.com/urfave/cli/v3"
)

func GenerateID(cCtx context.Context, cmd *cli.Command) error {
	n := cmd.Int("length")
	noNewline := cmd.Bool("no_newline")
	fmt.Print(idGen.New(n, noNewline))
	return nil

}

var idFlags []cli.Flag = []cli.Flag{
	&cli.IntFlag{
		Name:        "length",
		Aliases:     []string{"l"},
		Usage:       "length of the id",
		Required:    false,
		Value:       11,
	},
	&cli.BoolFlag{
		Name:        "no_newline",
		Aliases:     []string{"n"},
		Usage:       "no new line at the end of the id",
	},
}
