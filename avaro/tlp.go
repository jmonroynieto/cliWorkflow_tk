package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
)

// On ideapad hardware TLP treats the BAT0 stop threshold as a switch for conservation
// mode rather than as a percentage: 1 enables it, 0 disables it.
const tlpStopKey = "STOP_CHARGE_THRESH_BAT0"

// Whether TLP reapplies charge thresholds when the power source changes to battery. TLP
// compares this against the literal "1", so any other value disables the behaviour.
const tlpRestoreKey = "RESTORE_THRESHOLDS_ON_BAT"

// Locations of TLP's configuration, as variables so tests can point them at a fixture.
// tlpDefaults holds TLP's intrinsic defaults and is the lowest-precedence source; it is
// where RESTORE_THRESHOLDS_ON_BAT is set on a stock install.
var (
	tlpMainConf = "/etc/tlp.conf"
	tlpDropInD  = "/etc/tlp.d"
	tlpDefaults = "/usr/share/tlp/defaults.conf"
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

// readTLPSetting reports the effective conservation-mode assignment.
func readTLPSetting() tlpSetting {
	return readTLPKey(tlpStopKey)
}

// readTLPKey walks TLP's configuration in the order TLP itself does: the intrinsic
// defaults, then drop-ins from /etc/tlp.d in lexical order, then /etc/tlp.conf, with
// later assignments winning.
func readTLPKey(key string) tlpSetting {
	var result tlpSetting

	dropIns, err := filepath.Glob(filepath.Join(tlpDropInD, "*.conf"))
	if err != nil {
		logrus.Debug("tlp.d glob failed: ", err)
	}
	sort.Strings(dropIns)

	sources := append([]string{tlpDefaults}, dropIns...)
	for _, path := range append(sources, tlpMainConf) {
		value, ok := scanAssignment(path, key)
		if ok {
			result = tlpSetting{Found: true, Value: value, Source: path}
		}
	}
	return result
}

// restoreOnBattery reports whether TLP reapplies thresholds on a switch to battery, and
// says where the answer came from.
func restoreOnBattery() (enabled bool, value, source string) {
	s := readTLPKey(tlpRestoreKey)
	if !s.Found {
		// A stock install always assigns this in defaults.conf. If that file is missing
		// or unreadable, assume TLP's shipped value rather than claim it will leave a
		// flip standing.
		logrus.Debug("no ", tlpRestoreKey, " assignment found, assuming TLP's shipped 1")
		return true, "1", "TLP's built-in defaults"
	}
	return s.Value == "1", s.Value, s.Source
}

// reassertNote describes when TLP will put the configured state back. TLP reapplies
// thresholds only on a change of power source to battery, and only when
// RESTORE_THRESHOLDS_ON_BAT is 1; a switch to AC never reapplies them, which is what
// makes a flip-then-plug-in charge to 100% possible. Boot and `tlp start` reapply
// unconditionally. now is the live state, used to say whether a charge will run uncapped.
func reassertNote(configured, now bool) string {
	restore, value, source := restoreOnBattery()
	// The source path is interpolated on its own trailing line: it varies in length and
	// would otherwise wreck the wrapping of the sentence it sits in.
	provenance := fmt.Sprintf("(%s=%s in %s)\n", tlpRestoreKey, value, source)

	if !restore {
		return fmt.Sprintf(
			"TLP does not reapply thresholds when you unplug, so %s comes back only at boot\n"+
				"or on sudo tlp start.\n",
			stateWord(configured)) + provenance
	}

	note := fmt.Sprintf(
		"TLP puts %s back when the power source changes to battery - unplugging, or resuming\n"+
			"from suspend onto battery. Plugging in does not reapply it;\n"+
			"boot and sudo tlp start do.\n",
		stateWord(configured))
	if !now {
		note += "Until then charging is uncapped, so a charge started now runs to 100%.\n"
	}
	return note + provenance
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
