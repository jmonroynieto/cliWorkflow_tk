package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	wManagerHints "github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/xprop"
	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	Version  = "1.2.0"
	CommitId string
)

func getWindowName(xUtil *xgbutil.XUtil, window xproto.Window) string {
	name, err := wManagerHints.WmNameGet(xUtil, window)
	if err != nil {
		return ""
	}
	return name
}

func getWindowClass(xUtil *xgbutil.XUtil, window xproto.Window) string {
	windowClass, err := xprop.PropValStrs(xprop.GetProperty(xUtil, window, "WM_CLASS"))
	if err != nil || len(windowClass) < 2 {
		return ""
	}
	return windowClass[1]
}

func getExecutablePath(processID uint) string {
	executablePath, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", processID))
	if err != nil {
		return ""
	}
	return executablePath
}

func getExecutableBasename(executablePath string) string {
	return filepath.Base(executablePath)
}

func listOpenWindows(ctx context.Context, cmd *cli.Command) error {
	xConnection, cErr := xgb.NewConn()
	errorutils.ExitOnFail(cErr, errorutils.WithMsg("Failed to create X connection"))
	defer xConnection.Close()

	xUtil, err := xgbutil.NewConnXgb(xConnection)
	errorutils.ExitOnFail(err, errorutils.WithMsg("Failed to create XUtil connection"))

	clientWindows, err := wManagerHints.ClientListGet(xUtil)
	errorutils.ExitOnFail(err, errorutils.WithMsg("Failed to get client list"))

	for _, window := range clientWindows {
		windowName := getWindowName(xUtil, window)
		windowClass := getWindowClass(xUtil, window)

		processID, err := wManagerHints.WmPidGet(xUtil, window)
		if err != nil {
			logrus.Warnf("Failed to get PID for window %d: %v", window, err)
			continue
		}

		executablePath := getExecutablePath(uint(processID))
		executableBasename := getExecutableBasename(executablePath)

		fmt.Printf("Window Name: %s\n", windowName)
		fmt.Printf("Window Class: %s\n", windowClass)
		fmt.Printf("Executable Path: %s\n", executablePath)
		fmt.Printf("Executable Basename: %s\n", executableBasename)
		fmt.Println(strings.Repeat("-", 40))
	}
	return nil
}

func main() {
	err := (&cli.Command{

		Name:    "xwindows",
		Usage:   "list open windows",
		Version: fmt.Sprintf("%s (%s)", Version, CommitId),
		Action:  listOpenWindows,
	}).Run(context.Background(), os.Args)
	errorutils.ExitOnFail(err)
}
