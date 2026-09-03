package main

import (
	"os"
	"path/filepath"
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

	oldMain, oldDir := tlpMainConf, tlpDropInD
	tlpMainConf, tlpDropInD = conf, dir
	t.Cleanup(func() { tlpMainConf, tlpDropInD = oldMain, oldDir })
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
