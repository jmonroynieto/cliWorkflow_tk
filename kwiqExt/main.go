package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	filetyper "kwiqExt/fileTyper"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	Version  = "0.2"
	CommitId string
)

func init() {
	if errorutils.ToggleColor() { // ensure color output is disabled
		errorutils.ToggleColor()
	}
}

var app cli.Command = cli.Command{
	Name:        "kwiqExt",
	Description: "identify file types by category",
	Commands:    appCMDS,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug logging",
			Action: func(ctx context.Context, cmd *cli.Command, shouldDebug bool) error {
				if shouldDebug {
					logrus.SetLevel(logrus.DebugLevel)
				}
				return nil
			},
		},
	},
}

func main() {
	err := app.Run(context.Background(), os.Args)
	errorutils.ExitOnFail(err)
}

var appCMDS = []*cli.Command{
	{
		Name:        "type",
		Description: "identify category of filetype",
		Action:      general,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "long",
				Usage: "display long output including filename and mimetype",
				Aliases: []string{
					"l"},
			},
		},
	},
	{
		Name:        "mimetype",
		Description: "identify mimetype of file",
		Action:      mimetype,
	},
}

func general(ctx context.Context, cmd *cli.Command) error {

	for _, file := range cmd.Args().Slice() {
		if i, err := os.Stat(file); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				logrus.Warn("File not found: " + file)
			}
			continue
		} else if i.IsDir() {
			if cmd.Bool("long") {
				fmt.Printf("%s\t%s\n", file, "directory")
				continue
			}
			fmt.Println("DIR*")
			continue
		}
		f, err := filetyper.DetermineFMTtype(file)
		errorutils.ExitOnFail(err)
		if cmd.Bool("long") {
			o, err := os.Open(file)
			errorutils.ExitOnFail(err)
			fmt.Printf("%s\t%s\t%s\n", file, f, filetyper.GetHeader(o).MIME.Value)
			continue
		}
		fmt.Println(f)
	}
	return nil
}

func mimetype(ctx context.Context, cmd *cli.Command) error {
	for _, file := range cmd.Args().Slice() {
		f, err := os.Open(file)
		errorutils.ExitOnFail(err, errorutils.WithMsg("failed to open file "+file))
		fmt.Println(filetyper.GetHeader(f).MIME.Value)
		f.Close()
	}
	return nil
}
