package main

import (
	"context"
	"fmt"

	"github.com/jmonroynieto/cliWorkflow_tk/fickleFinger/idGen"
	"github.com/urfave/cli/v3"
)

func GenerateID(cCtx context.Context, cmd *cli.Command) error {
	l := cmd.Int64("length")
	noNewline := cmd.Bool("no_newline")
	fmt.Print(idGen.New(l, noNewline))
	return nil

}

var idFlags []cli.Flag = []cli.Flag{
	&cli.Int64Flag{
		Name:     "length",
		Aliases:  []string{"l"},
		Usage:    "length of the id",
		Required: false,
		Value:    11,
	},
	&cli.BoolFlag{
		Name:    "no_newline",
		Aliases: []string{"n"},
		Usage:   "no new line at the end of the id",
	},
}
