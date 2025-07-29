package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestSetWindow(t *testing.T) {
	// A fixed window height is assumed for all tests.
	const windowHeight = 5

	tests := []struct {
		name              string
		initialBuf        []string
		windowStartIdx    int // The index in the buffer where the window starts.
		bufSelectionIndex int // The index in the buffer for the selected line.
		wantBeforeLines   []string
		wantAfterLines    []string
		wantLineText      string
		expectedErr       error
	}{
		{
			name:              "Selection is in the middle of the window",
			initialBuf:        dutchNumbers,
			windowStartIdx:    10,
			bufSelectionIndex: 12, // "twaalf"
			wantBeforeLines:   []string{"tien", "elf"},
			wantAfterLines:    []string{"dertien", "veertien"},
			wantLineText:      "twaalf",
		},
		{
			name:              "Selection is at the top of the window",
			initialBuf:        dutchNumbers,
			windowStartIdx:    5,
			bufSelectionIndex: 5, // "vijf"
			wantBeforeLines:   []string{},
			wantAfterLines:    []string{"zes", "zeven", "acht", "negen"},
			wantLineText:      "vijf",
		},
		{
			name:              "Selection is at the bottom of the window",
			initialBuf:        dutchNumbers,
			windowStartIdx:    20,
			bufSelectionIndex: 24,
			wantBeforeLines:   []string{"twintig", "eenentwintig", "tweeëntwintig", "drieëntwintig"},
			wantAfterLines:    []string{},
			wantLineText:      "vierentwintig",
		},
		{
			name:              "Window is at the very start of the buffer",
			initialBuf:        dutchNumbers,
			windowStartIdx:    0,
			bufSelectionIndex: 2, // "twee"
			wantBeforeLines:   []string{"nul", "een"},
			wantAfterLines:    []string{"drie", "vier"},
			wantLineText:      "twee",
		},
		{
			name:              "Window is at the very end of the buffer (less than 5 lines)",
			initialBuf:        dutchNumbers,
			windowStartIdx:    len(dutchNumbers) - 3, // Starts at "achtentwintig"
			bufSelectionIndex: len(dutchNumbers) - 1, // "dertig"
			wantBeforeLines:   []string{"zesentwintig", "zevenentwintig", "achtentwintig", "negenentwintig"},
			wantAfterLines:    []string{},
			wantLineText:      "dertig",
		},
		// --- Edge Cases ---
		{
			name:              "Selection is outside (before) the window",
			initialBuf:        dutchNumbers,
			windowStartIdx:    10,
			bufSelectionIndex: 5, // Selection is not visible in the window pane.
			// Expected behavior: The window is displayed, but nothing is selected.
			wantBeforeLines: []string{},
			wantAfterLines:  []string{},
			wantLineText:    "",
			expectedErr:     OOB,
		},
		{
			name:              "Selection is outside (after) the window",
			initialBuf:        dutchNumbers,
			windowStartIdx:    10,
			bufSelectionIndex: 20, // Selection is not visible in the window pane.
			// Expected behavior: The window is displayed, but nothing is selected.
			wantBeforeLines: []string{},
			wantAfterLines:  []string{},
			wantLineText:    "",
			expectedErr:     OOB,
		},
		{
			name:              "the buf is 5 items long",
			initialBuf:        dutchNumbers[:5],
			windowStartIdx:    2,
			bufSelectionIndex: 2,
			wantBeforeLines:   []string{"nul", "een"},
			wantAfterLines:    []string{"drie", "vier"},
			wantLineText:      "twee",
		},
		{
			name:              "the buf is smaller than the window",
			initialBuf:        dutchNumbers[:4],
			windowStartIdx:    2, // even when selector is before the start of the buffer this should still work
			bufSelectionIndex: 1,
			wantBeforeLines:   []string{"nul"},
			wantAfterLines:    []string{"twee", "drie"},
			wantLineText:      "een",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := lines{
				completeBuf:    tc.initialBuf,
				bufSelectIndex: tc.bufSelectionIndex,
				BeforeLines:    make([]string, 2, 5),
				AfterLines:     make([]string, 2, 5),
				delIdx:         make([]bool, len(tc.initialBuf)),
			}

			var err error
			l, err = l.setWindow(tc.windowStartIdx)
			if err != nil && tc.expectedErr == nil {
				t.Error(err)
			} else if !errors.Is(err, tc.expectedErr) {
				t.Errorf("Error mismatch:\nwant: %q\ngot:  %q", tc.expectedErr, err)
			} else if err != nil {
				return
			}
			if !reflect.DeepEqual(l.BeforeLines, tc.wantBeforeLines) {
				t.Errorf("BeforeLines mismatch:\nwant: %q\ngot:  %q", tc.wantBeforeLines, l.BeforeLines)
			}

			if !reflect.DeepEqual(l.AfterLines, tc.wantAfterLines) {
				t.Errorf("AfterLines mismatch:\nwant: %q\ngot:  %q", tc.wantAfterLines, l.AfterLines)
			}

			if l.lineText != tc.wantLineText {
				t.Errorf("lineText mismatch:\nwant: %q\ngot:  %q", tc.wantLineText, l.lineText)
			}
		})
	}
}

var dutchNumbers = []string{
	"nul", "een", "twee", "drie", "vier", "vijf",
	"zes", "zeven", "acht", "negen", "tien",
	"elf", "twaalf", "dertien", "veertien", "vijftien",
	"zestien", "zeventien", "achttien", "negentien", "twintig",
	"eenentwintig", "tweeëntwintig", "drieëntwintig", "vierentwintig", "vijfentwintig",
	"zesentwintig", "zevenentwintig", "achtentwintig", "negenentwintig", "dertig",
}
