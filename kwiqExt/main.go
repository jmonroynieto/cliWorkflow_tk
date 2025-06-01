package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	filetyper "github.com/jmonroynieto/cliWorkflow_tk/kwiqExt/filetyper"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	Version  string
	Revision = ".2"
	CommitId string
)

func init() {
	if errorutils.ToggleColor() { // ensure color output is disabled
		errorutils.ToggleColor()
	}
}

var app cli.Command = cli.Command{
	Name:        "kwiqExt",
	Version:     fmt.Sprintf("%s (%s)", Version+Revision, CommitId),
	Description: "identify file types by category",
	Commands:    appCMDS,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"d"},
			Usage:   "enable debug logging",
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
			printerAid(cmd, file, filetyper.FmtType(222))
			continue
		} else {
			if i.Mode()&os.ModeNamedPipe != 0 || i.Mode()&os.ModeSocket != 0 || i.Mode()&os.ModeDevice != 0 {
				printerAid(cmd, file, filetyper.OTHER)
				continue
			}
		}
		f, err := filetyper.DetermineFMTtype(file)
		errorutils.ExitOnFail(err)
		printerAid(cmd, file, f)
	}
	return nil
}

func printerAid(cmd *cli.Command, path string, kind filetyper.FmtType) {
	if path == "" || kind.String() == "" {
		return
	}
	if cmd.Bool("long") {
		if kind == filetyper.OTHER {
			fmt.Printf("%s\t%s\n", path, "OTHER")
			return
		}
		o, err := os.Open(path)
		errorutils.ExitOnFail(err)
		fmt.Printf("%s\t%s\t%s\n", path, kind, filetyper.HeaderTest(o).MIME.Value)
		o.Close()
		return
	}
	fmt.Println(kind)
}

func mimetype(ctx context.Context, cmd *cli.Command) error {
	for _, file := range cmd.Args().Slice() {
		f, err := os.Open(file)
		errorutils.ExitOnFail(err, errorutils.WithMsg("failed to open file "+file))
		fmt.Println(filetyper.HeaderTest(f).MIME.Value)
		f.Close()
	}
	return nil
}
