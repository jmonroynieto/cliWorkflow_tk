package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
)

// sysfsGlob matches the ideapad_acpi driver's per-device attribute directory. The
// concrete node is usually VPC2004:00, but the ACPI id varies across models, so the
// driver directory is the stable anchor rather than the /sys/devices path it links to.
const sysfsGlob = "/sys/bus/platform/drivers/ideapad_acpi/*/conservation_mode"

// conservationPath resolves the single conservation_mode attribute exposed by the
// ideapad_laptop module.
func conservationPath() (string, error) {
	matches, err := filepath.Glob(sysfsGlob)
	if err != nil {
		return "", errorutils.New(err, errorutils.WithLineRef("lUplEqEFEO2"), errorutils.WithMsg("malformed sysfs glob"))
	}
	switch len(matches) {
	case 1:
		logrus.Debug("conservation_mode at ", matches[0])
		return matches[0], nil
	case 0:
		return "", errorutils.NewReport(
			"no conservation_mode attribute under "+sysfsGlob+"\n"+
				"\tthis machine is not a Lenovo ideapad, or the ideapad_laptop module is not loaded\n"+
				"\tcheck with: lsmod | grep ideapad_laptop",
			"oFFHtPTqQ86",
			errorutils.WithExitCode(1),
		)
	default:
		return "", errorutils.NewReport(
			fmt.Sprintf("expected one conservation_mode attribute, found %d: %s", len(matches), strings.Join(matches, ", ")),
			"a6dLExNTO13",
			errorutils.WithExitCode(1),
		)
	}
}

// readConservation returns the live embedded-controller state.
func readConservation(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, errorutils.New(err, errorutils.WithLineRef("Wa3UI05Z6SY"), errorutils.WithMsg("cannot read "+path))
	}
	switch v := strings.TrimSpace(string(raw)); v {
	case "1":
		return true, nil
	case "0":
		return false, nil
	default:
		return false, errorutils.NewReport(
			fmt.Sprintf("unexpected value %q in %s, wanted 0 or 1", v, path),
			"90ry9VPOOqO",
			errorutils.WithExitCode(1),
		)
	}
}

// writeConservation sets the attribute, escalating through sudo only when the process
// lacks write permission itself. Running as root, or with a udev rule granting group
// write, keeps the whole operation in-process.
func writeConservation(path string, on bool) error {
	value := "0"
	if on {
		value = "1"
	}

	err := os.WriteFile(path, []byte(value), 0o644)
	if err == nil {
		logrus.Debug("wrote ", value, " directly")
		return nil
	}
	if !errors.Is(err, os.ErrPermission) {
		return errorutils.New(err, errorutils.WithLineRef("uaMEUgWSdUO"), errorutils.WithMsg("cannot write "+path))
	}

	logrus.Debug("direct write denied, escalating via sudo")
	// tee is the only portable way to redirect into a root-owned file across a sudo
	// boundary; its echo of the value goes to /dev/null so status output stays clean.
	sudo := exec.Command("sudo", "tee", path)
	sudo.Stdin = strings.NewReader(value)
	sudo.Stdout = nil
	sudo.Stderr = os.Stderr
	if err := sudo.Run(); err != nil {
		return errorutils.New(err, errorutils.WithLineRef("Y4axxYwi5u4"), errorutils.WithMsg("sudo tee "+path+" failed"))
	}
	return nil
}

func stateWord(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func stateMeaning(on bool) string {
	if on {
		return "charging stops near 80%"
	}
	return "charges to 100%"
}
