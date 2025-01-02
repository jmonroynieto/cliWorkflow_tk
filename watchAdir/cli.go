package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var appFlags []cli.Flag = []cli.Flag{
	&cli.BoolFlag{
		Name:    "debug",
		Aliases: []string{"d"},
		Usage:   "activates debugging messages",
		Action: func(ctx context.Context, cli *cli.Command, shouldDebug bool) error {
			if shouldDebug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil
		},
	},
}
var timeout cli.StringFlag = cli.StringFlag{
	Name:  "timeout",
	Usage: "sets the timeout for the process. WATCHADIR never runs indefinitely. default 12h",
	Value: "12h",
	Action: func(ctx context.Context, cli *cli.Command, timeout string) error {
		var err error
		requestedTime, err = time.ParseDuration(timeout)
		return err
	},
}

var app = cli.Command{
	Name:    "watchAdir",
	Usage:   "Notifies when a directory is changed with printouts",
	Flags:   append(appFlags, &timeout),
	Version: fmt.Sprintf("%s - %s", Version, CommitId),
	Aliases: []string{"w", "walltime"},
	Action: func(ctx context.Context, cli *cli.Command) error {

		currDir, err := os.Getwd()
		errorutils.ExitOnFail(err, errorutils.WithMsg("failed to get current directory"))

		// Create a lock to protect the directory.
		lock := &sync.Mutex{}

		// Watch the current directory for new files.
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return err
		}
		defer watcher.Close()

		// Add the current directory to the watcher.
		err = watcher.Add(currDir)
		if err != nil {
			return err
		}

		// Start a goroutine to watch for new files.
		go func() {
			for {
				// Lock the directory.
				lock.Lock()
				defer lock.Unlock()

				// Watch for events.
				for event := range watcher.Events {
					switch event.Op {
					case fsnotify.Create:
						// A new file was created.
						fmt.Println("A new file named", event.Name, "was created on", time.Now().Format("January 2, 2006 at 15:04:05"))

					case fsnotify.Write:
						// A file was modified.
						fmt.Println("The file", event.Name, "was modified on", time.Now().Format("January 2, 2006 at 15:04:05"))

					case fsnotify.Remove:
						// A file was removed.
						fmt.Println("The file", event.Name, "was removed on", time.Now().Format("January 2, 2006 at 15:04:05"))
					default:
						continue
					}

				}
			}
		}()
		time.Sleep(requestedTime)
		return nil
	},
}
