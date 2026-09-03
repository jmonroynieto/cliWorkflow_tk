package main

import (
	"bufio"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
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

func TestIndex(t *testing.T) {

	type testCase struct {
		name        string
		input       string
		expected    index
		description string
	}

	var tests = []testCase{
		{
			name:        "Empty file",
			input:       "",
			expected:    index{0},
			description: "Test that an empty file returns an index with a single element, 0",
		},
		{
			name:        "Single line",
			input:       "Hello\n",
			expected:    index{0, 6},
			description: "Test that a single line returns an index with two elements, 0 and the length of the line",
		},
		{
			name:        "Multiple lines",
			input:       "Hello\nWorld\n",
			expected:    index{0, 6, 12},
			description: "Test that multiple lines return an index with the correct lengths",
		},
		{
			name:        "Lines with different lengths",
			input:       "a\nHello\nWorld\n",
			expected:    index{0, 2, 8, 14},
			description: "Test that lines with different lengths return an index with the correct lengths",
		},
		{
			name:        "No newline at the end",
			input:       "Hello\nWorld",
			expected:    index{0, 6, 12}, // the last byte is a fiction since there is no new line, the program will account for that.
			description: "Test that a file without a newline at the end returns an index with the correct lengths",
		},
		{
			name:        "Only newline characters",
			input:       "\n\n\n",
			expected:    index{0, 1, 2, 3},
			description: "Test that a file with only newline characters returns an index with the correct lengths",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := indexLines(strings.NewReader(tc.input))
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("indexLines() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestReadat(t *testing.T) {
	// test no newline at the end, since the case for that in the index test reveals that the end point is hard coded to be one-off from the last character. Hoewever using the scanner seems to handle that correctly

	//make dummy file where line numbers correspond to the content
	contents := strings.NewReader(strings.Trim(strings.Join(dutchNumbers[1:], "\n"), " \n"))
	file, err := os.CreateTemp("", "testfile")
	if err != nil {
		logrus.Warn(err)
		t.Fail()
	}
	defer os.Remove(file.Name())
	defer file.Close()
	_, err = io.Copy(file, contents)
	if err != nil {
		logrus.Warn(err)
		t.Fail()
	}
	//seek file and contents to beginning
	_, err = file.Seek(0, 0)
	if err != nil {
		logrus.Warn(err)
		t.Fail()
	}
	_, err = contents.Seek(0, 0)
	if err != nil {
		logrus.Warn(err)
		t.Fail()
	}
	idx := indexLines(contents)
	if err != nil {
		logrus.Warn(err)
		t.Fail()
	}
	// main event
	lines, err := idx.readlines(file, 20, 30)
	if err != nil {
		logrus.Warn(err)
		t.Fail()
	}
	expected := dutchNumbers[20:31]
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("Readat() = %v, want %v", lines, expected)
	}
}

func TestDutchNumberScanner(t *testing.T) {
	inputString := "nul\neen\ntwee\ndrie\nvier\nvijf\nzes\nzeven\nacht\nnegen\ntien\nelf\ntwaalf\ndertien\nveertien\nvijftien\nzestien\nzeventien\nachttien\nnegentien\ntwintig\neenentwintig\ntweeëntwintig\ndrieëntwintig\nvierentwintig\nvijfentwintig\nzesentwintig\nzevenentwintig\nachtentwintig\nnegenentwintig\ndertig"

	expectedLines := dutchNumbers

	scanner := bufio.NewScanner(strings.NewReader(inputString))

	actualLines := []string{}
	for scanner.Scan() {
		actualLines = append(actualLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}

	if len(actualLines) != len(expectedLines) {
		t.Fatalf("Mismatch in number of lines. Expected %d, got %d. Actual: %v", len(expectedLines), len(actualLines), actualLines)
	}

	for i := range expectedLines {
		if actualLines[i] != expectedLines[i] {
			t.Errorf("Line %d mismatch. Expected '%s', got '%s'", i, expectedLines[i], actualLines[i])
		}
	}
}
