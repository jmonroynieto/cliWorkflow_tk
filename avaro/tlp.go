package main

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
)

// On ideapad hardware TLP treats the BAT0 stop threshold as a switch for conservation
// mode rather than as a percentage: 1 enables it, 0 disables it.
const tlpStopKey = "STOP_CHARGE_THRESH_BAT0"

// Locations of TLP's configuration, as variables so tests can point them at a fixture.
var (
	tlpMainConf = "/etc/tlp.conf"
	tlpDropInD  = "/etc/tlp.d"
)

// tlpSetting is the effective STOP_CHARGE_THRESH_BAT0 assignment, with the file it
// came from. Found is false when no uncommented assignment exists anywhere.
type tlpSetting struct {
	Found  bool
	Value  string
	Source string
}

// Enabled reports whether the configured value asks for conservation mode. Anything
// other than the documented 0/1 is reported as unrecognised by the caller rather than
// coerced, since TLP's meaning for other values is model-dependent.
func (s tlpSetting) Enabled() (on bool, recognised bool) {
	switch s.Value {
	case "1":
		return true, true
	case "0":
		return false, true
	default:
		return false, false
	}
}

// readTLPSetting walks TLP's configuration in the order TLP itself does: drop-ins from
// /etc/tlp.d in lexical order, then /etc/tlp.conf, with later assignments winning.
func readTLPSetting() tlpSetting {
	var result tlpSetting

	dropIns, err := filepath.Glob(filepath.Join(tlpDropInD, "*.conf"))
	if err != nil {
		logrus.Debug("tlp.d glob failed: ", err)
	}
	sort.Strings(dropIns)

	for _, path := range append(dropIns, tlpMainConf) {
		value, ok := scanAssignment(path, tlpStopKey)
		if ok {
			result = tlpSetting{Found: true, Value: value, Source: path}
		}
	}
	return result
}

// scanAssignment returns the last uncommented `key=value` in path.
func scanAssignment(path, key string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		logrus.Debug("skipping ", path, ": ", err)
		return "", false
	}
	defer f.Close()

	var value string
	var found bool
	prefix := key + "="
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || !strings.HasPrefix(line, prefix) {
			continue
		}
		value = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, prefix)), `"'`)
		found = true
	}
	if err := scanner.Err(); err != nil {
		logrus.Debug("read error in ", path, ": ", err)
	}
	return value, found
}
