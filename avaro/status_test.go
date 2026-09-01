package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn with stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = old
	w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestReportTLPDivergence(t *testing.T) {
	tests := []struct {
		name       string
		main       string
		live       bool
		wantWarn   bool
		wantSubstr string
	}{
		{
			name:       "agreement is quiet",
			main:       "STOP_CHARGE_THRESH_BAT0=1\n",
			live:       true,
			wantSubstr: "-> on",
		},
		{
			name:       "config on, controller off",
			main:       "STOP_CHARGE_THRESH_BAT0=1\n",
			live:       false,
			wantWarn:   true,
			wantSubstr: "configured for on but the controller is off",
		},
		{
			name:       "config off, controller on",
			main:       "STOP_CHARGE_THRESH_BAT0=0\n",
			live:       true,
			wantWarn:   true,
			wantSubstr: "configured for off but the controller is on",
		},
		{
			name:       "no assignment says nothing reasserts",
			live:       true,
			wantSubstr: "nothing will reassert this",
		},
		{
			name:       "unrecognised value is reported as such",
			main:       "STOP_CHARGE_THRESH_BAT0=80\n",
			live:       true,
			wantSubstr: "unrecognised",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withFixture(t, tc.main, nil)

			out := captureStdout(t, func() { reportTLP(tc.live) })
			if !strings.Contains(out, tc.wantSubstr) {
				t.Errorf("output missing %q, got:\n%s", tc.wantSubstr, out)
			}
			if warned := strings.Contains(out, "divergence:"); warned != tc.wantWarn {
				t.Errorf("divergence warning = %v, want %v, got:\n%s", warned, tc.wantWarn, out)
			}
		})
	}
}

func TestWarnTLPWillRevert(t *testing.T) {
	withFixture(t, "STOP_CHARGE_THRESH_BAT0=1\n", nil)

	// Flipped to off while TLP asks for on: the change is cosmetic.
	out := captureStdout(t, func() { warnTLPWillRevert(false) })
	if !strings.Contains(out, "this will not stick") {
		t.Errorf("expected a revert warning, got:\n%s", out)
	}
	if !strings.Contains(out, "STOP_CHARGE_THRESH_BAT0=0") {
		t.Errorf("expected the corrective assignment, got:\n%s", out)
	}

	// Flipped to on, which is what TLP already asks for: nothing to say.
	out = captureStdout(t, func() { warnTLPWillRevert(true) })
	if out != "" {
		t.Errorf("expected silence when config agrees, got:\n%s", out)
	}
}
