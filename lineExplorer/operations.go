package main

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
)

func title(line uint32) string {
	x1 := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).AlignHorizontal(lipgloss.NoTabConversion).Render("selected: "+strconv.Itoa(int(line))) + "\n"
	x2 := lipgloss.NewStyle().Foreground(lipgloss.Color("#CACACA")).Render("—————")
	x := x1 + x2
	return x
}
func (m Model) shuffleAndSetLines() (Model, tea.Cmd) {
	m.currentLine = rand.Uint32N(uint32(len(m.idx))) //random 1-indexed line to sample (first element is always 0)
	sL, eL, relativePos := bounds(m.currentLine, uint32(len(m.idx)))
	var err error
	m.buf.completeBuf, err = m.idx.readlines(m.file, sL, eL)
	errorutils.ExitOnFail(err)
	m.buf.delIdx = make([]bool, len(m.buf.completeBuf))
	m.buf.bufSelectIndex = relativePos
	m.buf, err = m.buf.setWindow(relativePos - 2)
	m.buf.ogLineFileinx = int(m.currentLine)
	m.buf.ogLineBufIndex = m.buf.bufSelectIndex
	m.buf, err = m.buf.setWindow(m.buf.bufSelectIndex - 2)
	m.buf.delIdx = make([]bool, len(m.buf.completeBuf))
	for i := sL; i <= eL; i++ {
		_, ok := m.shouldDelete[i]
		if ok {
			m.buf.delIdx[i-sL] = true
		}
	}
	m.buf.gutterLines = m.buf.updateGutter()
	return m, nil
}

// deals in line numbers 1-indexed
func bounds(currentLine, linesInFile uint32) (startLine, endLine uint32, relativePosition int) {
	relativePos := min(15, currentLine)
	startLine = currentLine - relativePos
	endLine = currentLine + 15

	if currentLine > uint32(linesInFile-15) {
		endLine = uint32(linesInFile)
	}

	return startLine, endLine, int(relativePos)
}

type lines struct {
	unselectedItemStyle lipgloss.Style
	selectedItemStyle   lipgloss.Style
	ogLineFileinx       int
	ogLineBufIndex      int
	completeBuf         []string
	bufSelectIndex      int // item to highlight and work on
	BeforeLines         []string
	lineText            string
	AfterLines          []string
	delIdx              []bool
	gutterLines         []string
}

func (l lines) String() string {
	var B string
	var A string

	for _, v := range l.BeforeLines {
		if v == "" || v == " " || v == "\n" {
			continue
		}
		B += v + "\n"
	}
	for _, v := range l.AfterLines {
		if v == "" {
			continue
		}
		A += v + "\n"
	}
	var parts = make([]string, 0, 3)
	for i, part := range []string{B, l.lineText, A} {
		part = strings.TrimSuffix(part, "\n")
		if part == "" {
			continue
		}
		switch i {
		case 0, 2:
			part = l.unselectedItemStyle.Render(part)
		case 1:
			part = l.selectedItemStyle.Render(part)
		}
		parts = append(parts, part)
	}
	var deleteGutter strings.Builder
	for i, v := range l.gutterLines {
		deleteGutter.WriteString(v)
		if i < len(l.gutterLines)-1 {
			deleteGutter.WriteString("\n")
		}
	}
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		deleteGutter.String(),
		lipgloss.JoinVertical(
			lipgloss.NoTabConversion,
			parts...,
		))
}

var OOB = errors.New("selected index out of bounds")

// setWindow changes the loaded lines but not the buffer
//
// windowStart is the buffer index at which we want to start.
// Target window (5 lines) is adjusted to fit the buffer and its tips
func (l lines) setWindow(windowStart int) (lines, error) {
	maxIndex := len(l.completeBuf) - 1
	if maxIndex < 5 || windowStart < 0 {
		windowStart = 0
	} else {
		windowStart = min(windowStart, maxIndex-4)
	}

	selected := l.bufSelectIndex
	if selected < windowStart || selected > windowStart+4 || selected > maxIndex { //safety checks
		return l, errorutils.NewReport(fmt.Sprintf("Picked: %d WindowStart: %d MaxIndex: %d\n", selected, maxIndex, windowStart), "08MSWPlLllG", errorutils.WithInner(OOB))
	}
	relPtr := selected - windowStart

	var returnable error
	// capture values during a panic
	defer func() {
		if err := recover(); err != nil {
			logrus.Info("recovered from panic: ", err)
			logrus.Infof("[start: %d, bufSelection: %d, relPtr: %d, maxIndex: %d ]\n", windowStart, l.bufSelectIndex, relPtr, maxIndex)
			logrus.Infof("bounds of after[%d:%d]\n", min(windowStart+relPtr+2, len(l.completeBuf)), max(windowStart+relPtr+3, min(windowStart+5, len(l.completeBuf))))
			returnable = errorutils.NewReport("recovered from panic: "+err.(error).Error(), "7NEKdcE8loi")
		}
	}()

	// [(windowStart, beforelinesEnd), selectedLineIndex, (afterLinesStart, windowEnd)] are all the required indexes
	beforelinesEnd := min(windowStart+relPtr, len(l.completeBuf))
	selectedLineIndex := windowStart + relPtr
	afterlinesStart := min(windowStart+relPtr+1, len(l.completeBuf))
	windowEnd := min(windowStart+5, len(l.completeBuf)) // slice top is exclusive

	l.BeforeLines = make([]string, beforelinesEnd-windowStart, 5)
	l.AfterLines = make([]string, windowEnd-afterlinesStart, 5)
	copy(l.BeforeLines[:], l.completeBuf[windowStart:beforelinesEnd])
	l.lineText = l.completeBuf[selectedLineIndex]
	copy(l.AfterLines[:], l.completeBuf[afterlinesStart:windowEnd])
	l.gutterLines = l.updateGutter()
	return l, returnable
}

func (l lines) selectedFileIndex() int {
	return l.ogLineFileinx - (l.ogLineBufIndex - l.bufSelectIndex)
}
func (l lines) down() (lines, tea.Cmd) {
	var err error
	l.bufSelectIndex++
	if l.bufSelectIndex >= len(l.completeBuf) {
		l.bufSelectIndex = len(l.completeBuf) - 1
	}
	l, err = l.setWindow(l.bufSelectIndex - 2)
	if err != nil {
		return l, tea.Quit
	}
	return l, nil
}

func (l lines) up() (lines, tea.Cmd) {
	var err error
	l.bufSelectIndex--
	if l.bufSelectIndex < 0 {
		l.bufSelectIndex = 0
	}
	l, err = l.setWindow(l.bufSelectIndex - 2)
	if err != nil {
		return l, func() tea.Msg { return closeMsg{err} }
	}
	return l, nil
}

func (l lines) updateGutter() []string {
	maxIndex := len(l.completeBuf) - 1
	if maxIndex < 0 {
		return []string{}
	}
	windowStart := max(l.bufSelectIndex-2, 0)
	if maxIndex < 5 || windowStart < 0 {
		windowStart = 0
	} else {
		windowStart = min(windowStart, maxIndex-4)
	}
	adjustment := 1
	if l.lineText == "" {
		adjustment = 0
	}
	windowEnd := min(windowStart+5, len(l.completeBuf))
	l.gutterLines = make([]string, len(l.BeforeLines)+len(l.AfterLines)+adjustment)
	//using the indexes to set the gutter lines from l.delIdx
	for i, v := range l.delIdx[windowStart:windowEnd] {
		if v {
			l.gutterLines[i] = lipgloss.NewStyle().Foreground(lipgloss.Color("#F55536")).Render("x")
		} else {
			l.gutterLines[i] = " "
		}
	}
	return l.gutterLines
}
