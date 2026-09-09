package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withFixture points the TLP readers at a temporary tree. Each entry in dropIns is a
// file name under the drop-in directory; main is the content of tlp.conf, skipped when
// empty.
func withFixture(t *testing.T, main string, dropIns map[string]string) {
	t.Helper()
	root := t.TempDir()

	dir := filepath.Join(root, "tlp.d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range dropIns {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	conf := filepath.Join(root, "tlp.conf")
	if main != "" {
		if err := os.WriteFile(conf, []byte(main), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Point the defaults at a path inside the fixture too, so a test never reads the
	// host's /usr/share/tlp/defaults.conf. Absent unless a test writes it.
	defaults := filepath.Join(root, "defaults.conf")

	oldMain, oldDir, oldDefaults := tlpMainConf, tlpDropInD, tlpDefaults
	tlpMainConf, tlpDropInD, tlpDefaults = conf, dir, defaults
	t.Cleanup(func() { tlpMainConf, tlpDropInD, tlpDefaults = oldMain, oldDir, oldDefaults })
}

func TestReadTLPSettingPrecedence(t *testing.T) {
	tests := []struct {
		name       string
		main       string
		dropIns    map[string]string
		wantFound  bool
		wantValue  string
		wantOn     bool
		recognised bool
	}{
		{
			name:       "main conf only",
			main:       "STOP_CHARGE_THRESH_BAT0=1\n",
			wantFound:  true,
			wantValue:  "1",
			wantOn:     true,
			recognised: true,
		},
		{
			name:       "main conf overrides drop-in",
			main:       "STOP_CHARGE_THRESH_BAT0=0\n",
			dropIns:    map[string]string{"10-batt.conf": "STOP_CHARGE_THRESH_BAT0=1\n"},
			wantFound:  true,
			wantValue:  "0",
			recognised: true,
		},
		{
			name: "later drop-in wins",
			dropIns: map[string]string{
				"00-a.conf": "STOP_CHARGE_THRESH_BAT0=0\n",
				"99-z.conf": "STOP_CHARGE_THRESH_BAT0=1\n",
			},
			wantFound:  true,
			wantValue:  "1",
			wantOn:     true,
			recognised: true,
		},
		{
			name:      "commented assignment is not a setting",
			main:      "# STOP_CHARGE_THRESH_BAT0=1\n",
			wantFound: false,
		},
		{
			name:      "unrelated battery key ignored",
			main:      "STOP_CHARGE_THRESH_BAT1=80\n",
			wantFound: false,
		},
		{
			name:       "quoted value is unwrapped",
			main:       "STOP_CHARGE_THRESH_BAT0=\"1\"\n",
			wantFound:  true,
			wantValue:  "1",
			wantOn:     true,
			recognised: true,
		},
		{
			name:      "percentage value is not coerced",
			main:      "STOP_CHARGE_THRESH_BAT0=80\n",
			wantFound: true,
			wantValue: "80",
		},
		{
			name:      "no config at all",
			wantFound: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withFixture(t, tc.main, tc.dropIns)

			got := readTLPSetting()
			if got.Found != tc.wantFound {
				t.Fatalf("Found = %v, want %v", got.Found, tc.wantFound)
			}
			if !tc.wantFound {
				return
			}
			if got.Value != tc.wantValue {
				t.Errorf("Value = %q, want %q", got.Value, tc.wantValue)
			}
			on, recognised := got.Enabled()
			if recognised != tc.recognised {
				t.Errorf("recognised = %v, want %v", recognised, tc.recognised)
			}
			if on != tc.wantOn {
				t.Errorf("Enabled = %v, want %v", on, tc.wantOn)
			}
		})
	}
}

func TestRestoreOnBatteryFallsBackToShippedDefault(t *testing.T) {
	withFixture(t, "", nil)

	enabled, value, source := restoreOnBattery()
	if !enabled || value != "1" {
		t.Errorf("restoreOnBattery = %v/%q, want true/\"1\" when no assignment exists", enabled, value)
	}
	if source == tlpMainConf {
		t.Errorf("source = %q, want the built-in default", source)
	}
}

func TestRestoreOnBatteryPrecedence(t *testing.T) {
	tests := []struct {
		name    string
		main    string
		want    bool
		wantVal string
	}{
		{name: "explicitly off", main: "RESTORE_THRESHOLDS_ON_BAT=0\n", want: false, wantVal: "0"},
		{name: "explicitly on", main: "RESTORE_THRESHOLDS_ON_BAT=1\n", want: true, wantVal: "1"},
		// TLP compares against the literal 1, so anything else disables the behaviour.
		{name: "any other value is off", main: "RESTORE_THRESHOLDS_ON_BAT=yes\n", want: false, wantVal: "yes"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withFixture(t, tc.main, nil)

			enabled, value, _ := restoreOnBattery()
			if enabled != tc.want || value != tc.wantVal {
				t.Errorf("restoreOnBattery = %v/%q, want %v/%q", enabled, value, tc.want, tc.wantVal)
			}
		})
	}
}

func TestReassertNoteWording(t *testing.T) {
	// Restore enabled and the controller uncapped: the note must say unplugging brings
	// the cap back and that plugging in does not, since the whole point of a flip before
	// plugging in is that the charge runs past the cap.
	withFixture(t, "", nil)
	note := reassertNote(true, false)
	for _, want := range []string{"changes to battery", "Plugging in does not", "runs to 100%"} {
		if !strings.Contains(note, want) {
			t.Errorf("note missing %q, got:\n%s", want, note)
		}
	}

	// Live state already capped: no claim about an uncapped charge.
	if note := reassertNote(true, true); strings.Contains(note, "runs to 100%") {
		t.Errorf("unexpected uncapped-charge claim while capped, got:\n%s", note)
	}

	// Restore disabled: unplugging is not a reassertion event at all.
	withFixture(t, "RESTORE_THRESHOLDS_ON_BAT=0\n", nil)
	note = reassertNote(true, false)
	if !strings.Contains(note, "does not reapply thresholds when you unplug") {
		t.Errorf("expected the no-restore wording, got:\n%s", note)
	}
}
