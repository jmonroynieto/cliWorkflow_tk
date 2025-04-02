package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	Version       string
	Revision      = ".0"
	CommitId      string
	requestedTime time.Duration
)

var app = cli.Command{
	Name:    "watchAdir",
	Usage:   "Notifies when a directory is changed with printouts",
	Flags:   appFlags,
	Version: fmt.Sprintf("%s (%s)", Version+Revision, CommitId),
	Action:  vidi,
}

func main() {
	deferErr := app.Run(context.Background(), os.Args)
	errorutils.WarnOnFail(deferErr, errorutils.WithMsg("app failed execution"))
}

var appFlags []cli.Flag = []cli.Flag{
	&cli.BoolFlag{
		Name:    "debug",
		Aliases: []string{"D"},
		Usage:   "activates debugging messages",
		Action: func(ctx context.Context, cli *cli.Command, shouldDebug bool) error {
			if shouldDebug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil
		},
	},
	&cli.StringFlag{
		Name:    "timeout",
		Usage:   "sets the timeout for the process. WATCHADIR never runs indefinitely. default 20min",
		Aliases: []string{"walltime", "t", "w"},
		Value:   "20m",
	},
	&cli.IntFlag{
		Name:    "depth",
		Aliases: []string{"d"},
		Usage:   "Sets levels of directory to watch. It can watch directories that alread exist, -1 traverses the whole dir structure. default: 0",
		Value:   0,
	},
}

func vidi(ctx context.Context, cmd *cli.Command) error {
	var err error
	requestedTime, err = time.ParseDuration(cmd.String("timeout"))
	errorutils.ExitOnFail(err, errorutils.WithMsg("unit suffixes should be SI units"))
	if requestedTime <= 0 {
		return errorutils.NewReport("Zero time is a no-op comand", "7aEUnYgPNQf")
	}

	currDir, err := os.Getwd()
	errorutils.ExitOnFail(err, errorutils.WithMsg("failed to get current directory"))
	watcher, err := fsnotify.NewWatcher()
	errorutils.ExitOnFail(err)
	defer watcher.Close()

	paths, err := getPathsToWatch(currDir, int(cmd.Int("depth")))
	errorutils.ExitOnFail(err)
	for _, target := range paths {
		err = watcher.Add(target)
	}
	if err != nil {
		return err
	}

	go func() {
		for {
			// Watch for events.
			for event := range watcher.Events {
				switch event.Op {
				case fsnotify.Create:
					//check if it is a directory
					if info, _ := os.Stat(event.Name); info.IsDir() {
						fmt.Println("A new directory named", event.Name, "was created on", time.Now().Format("January 2, 2006 at 15:04:05"))
						if d := cmd.Int("depth"); d > 0 || d == -1 {
							newPaths, err := getPathsToWatch(event.Name, int(d)-1)
							errorutils.WarnOnFail(err, errorutils.WithMsg("failed to get paths to watch for new directory"))
							for _, target := range newPaths {
								err = watcher.Add(target)
								errorutils.WarnOnFail(err, errorutils.WithMsg("failed to watch new directory"))
							}
						}
						continue
					}
					fmt.Println("A new file named", event.Name, "was created on", time.Now().Format("January 2, 2006 at 15:04:05"))

				case fsnotify.Write:
					// A file was modified.
					fmt.Println("The file", event.Name, "was modified on", time.Now().Format("January 2, 2006 at 15:04:05"))

				case fsnotify.Remove:
					// A file was removed.
					fmt.Println("The file or directory ", event.Name, "was removed on", time.Now().Format("January 2, 2006 at 15:04:05"))
				default:
					continue
				}

			}
		}
	}()

	time.Sleep(requestedTime)
	return nil
}

func getPathsToWatch(path string, depth int) ([]string, error) {
	var paths []string
	if depth == 0 {
		return []string{path}, nil
	}
	paths = append(paths, path) //includes the current path
	entries, err := os.ReadDir(path)
	logrus.Debugf("entries in %s: %#v", path, entries)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			subPaths, err := getPathsToWatch(filepath.Join(path, entry.Name()), depth-1)
			if err != nil {
				return nil, err
			}
			paths = append(paths, subPaths...)
		}
	}
	return paths, nil
}
