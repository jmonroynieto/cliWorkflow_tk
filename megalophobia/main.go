package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pydpll/errorutils"
	"github.com/urfave/cli/v3"
)

var app = cli.Command{
	Name:        "megalophobia",
	Description: "Makes a three line window to display info, input is meant to be human paced. No scrolling",
	Action:      tool,
	Commands: []*cli.Command{
		{
			Name:        "demo",
			Description: "show example output to see behaviour",
			Action:      phobia,
		},
	},
}

func main() {
	err := app.Run(context.Background(), os.Args)
	errorutils.ExitOnFail(err)
}

func tool(ctx context.Context, cmd *cli.Command) error {
	var running bool = true
	err := SetupCapture()
	errorutils.ExitOnFail(err)
	defer TeardownCapture() // returns stdout to original value and clears the display window
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, syscall.SIGINT)

	go func() {
		for sig := range interrupted {
			if sig == syscall.SIGINT {
				running = false
			}
		}
	}()
	var userinput string
	for _, err := fmt.Scanln(&userinput); running && err == nil {
		fmt.Println(userinput)
	}
	if err != nil {
			fmt.Printf("\033[31mmegalophobia error: \033[0m%s", err.Error())
			fmt.Printf("input: %#v\n", userinput)
		}
	return nil
}
