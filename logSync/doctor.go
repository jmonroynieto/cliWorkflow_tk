package main

import (
	"context"
	"fmt"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/adbx"
	"github.com/urfave/cli/v3"
)

var doctorCommand = &cli.Command{
	Name:  "doctor",
	Usage: "check adb connectivity and whether mtime survives a push/pull round trip",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		cfg, err := loadConfig(cmd.String("config"))
		if err != nil {
			return err
		}

		report, err := adbx.Doctor(adbx.DoctorConfig{
			DeviceSerial: cfg.DeviceSerial,
			RemoteDirs:   remoteDirChecks(cfg),
		})
		printDoctorReport(report)
		if err != nil {
			return err
		}
		if !report.AllDirsExist() {
			fmt.Println("run `logSync primeMobile` to create the missing directories on the device")
		}
		if !report.ServerReachable || !report.AllDirsExist() {
			return fmt.Errorf("doctor: not ready to sync, see notes above")
		}
		return nil
	},
}

func printDoctorReport(r adbx.DoctorReport) {
	fmt.Printf("adb server reachable: %v\n", r.ServerReachable)
	if r.DeviceSerial != "" {
		fmt.Printf("device:               %s\n", r.DeviceSerial)
	}
	for _, d := range r.Dirs {
		fmt.Printf("%-16s exists: %v  (%s)\n", d.Key, d.Exists, d.Path)
	}
	if r.ProbedDir != "" {
		fmt.Printf("push/pull round trip: %v\n", r.ProbeRoundTrip)
		fmt.Printf("mtime survives:       %v\n", r.MtimeSurvives)
		if r.FreeKnown {
			fmt.Printf("free on device:       %s\n", humanBytes(r.FreeBytes))
		}
	}
	for _, n := range r.Notes {
		fmt.Printf("note: %s\n", n)
	}
}
