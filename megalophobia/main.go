package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/pprof"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	CommitID string
)

func init() {
	if os.Getenv("DEBUG_MODE") == "true" {
		err := func(filename string) error {
			f, err := os.Open(filename)
			if err != nil {
				panic(err)
			}
			os.Stdin = f
			return nil
		}("test2.txt")
		if err != nil {
			panic(err)
		}
	}
}

var app = cli.Command{
	Name:        "megalophobia",
	Description: "Makes a three line window to display info, input is meant to be human paced. No scrolling",
	Action:      tool,
	Version:     "v0.0.1 - commit: " + CommitID,
	Commands: []*cli.Command{
		{
			Name:        "demo",
			Description: "show example output to see behaviour",
			Action:      phobia,
		},
	},
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"d"},
			Usage:   "Enable debug logging",
			Action: func(c context.Context, cmd *cli.Command, b bool) error {
				if b {
					logrus.SetLevel(logrus.DebugLevel)
					logrus.Debug("Debug logging enabled")
				}
				return nil
			},
		},
	},
}

func main() {
	go func() {
		log.Println(http.ListenAndServe(":6060", nil))
	}()
	// Create a CPU profile file
	f, err := os.Create("profile.prof")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// Start CPU profiling
	if err := pprof.StartCPUProfile(f); err != nil {
		panic(err)
	}
	err = app.Run(context.Background(), os.Args)
	errorutils.ExitOnFail(err)
}

func tool(ctx context.Context, cmd *cli.Command) error {
	running, err := SetupCapture()
	errorutils.ExitOnFail(err, errorutils.WithMsg("Failed to setup capture: "))
	//reading loop
	scanner := bufio.NewScanner(os.Stdin)
	for *running && scanner.Scan() {
		userinput := scanner.Text()
		fmt.Println(userinput)
	}
	FinishCapture() //necessary to run: blocks main goroutineCapture()
	return nil
}
