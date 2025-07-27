package main

import (
	"bytes"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
)

func New(file *os.File, idx index) Model {
	h := help.New()
	h.ShortSeparator = " \x1b[94m|\x1b[0m "

	return Model{
		file:         file,
		idx:          idx,
		shouldDelete: make(map[uint32]struct{}),
		keymap:       defaultKeymap(),
		buf: lines{
			unselectedItemStyle: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B69AA6", Dark: "#4F6367"}),
			selectedItemStyle:   lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#D6BA7C"}),
		},
		help: h,
	}
}

type Model struct {
	logBuffer    *bytes.Buffer
	currentLine  uint32
	file         *os.File
	idx          index
	shouldDelete map[uint32]struct{}
	buf          lines //max renders 2 + 1 + 2 but keeps 30 in memory
	keymap       keymap
	help         help.Model
	// delete cross #F55536
}

func (m Model) Init() tea.Cmd { return func() tea.Msg { return shuffleMsg{} } }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, nil
	case shuffleMsg:
		return m.swapLine()
	case tea.KeyMsg:
		km := m.keymap
		errCMD := tea.Cmd(nil)
		switch {
		case key.Matches(msg, km.Down):
			m.buf, errCMD = m.buf.down()
			if errCMD != nil {
				return m, errCMD
			}
		case key.Matches(msg, km.Up):
			m.buf, errCMD = m.buf.up()
			if errCMD != nil {
				return m, errCMD
			}
		default:
			return m, tea.Quit
			/*case key.Matches(msg, km.Delete):
				m.shouldDelete[m.currentLine] = struct{}{}
			case key.Matches(msg, km.Shuffle):
				newLines := new(lines)
				populatelines(m.file, newLine, &(m.idx), newLines)

			case key.Matches(msg, km.Abort):
			case key.Matches(msg, km.Submit): */
		}
	}
	return m, nil
}

type shuffleMsg struct{}

func (m Model) View() string {
	if m.currentLine == 0 {
		return "loading..."
	}
	x, err := m.buf.setWindow(int(m.buf.rltvBufSelection) - 2)
	if err != nil {
		return "error..."
	}
	return title(m.currentLine) + x.String() + "\n\n" + m.help.View(m.keymap)
}

func title(line uint32) string {
	x1 := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).AlignHorizontal(lipgloss.NoTabConversion).Render("selected: "+strconv.Itoa(int(line))) + "\n"
	x2 := lipgloss.NewStyle().Foreground(lipgloss.Color("#CACACA")).Render("—————") + "\n"
	x := x1 + x2
	return x
}
func (m Model) swapLine() (Model, tea.Cmd) {
	m.currentLine = rand.Uint32N(uint32(len(m.idx)))
	var (
		startLine,
		endLine,
		relativePos uint32 // index at which the current line is located, useful for buffers that cannot get the 15 lines padding
	)
	// define buffer bounds
	if m.currentLine > 15 {
		startLine = m.currentLine - 15
		relativePos = 15
	} else {
		startLine = 0
		relativePos = m.currentLine
	}
	if m.currentLine <= uint32(len(m.idx)-15) {
		endLine = m.currentLine + 15
	} else {
		endLine = uint32(len(m.idx))
	}
	bufLines, err := m.idx.readlines(m.file, startLine, endLine)
	errorutils.ExitOnFail(err)
	m.buf.completeBuf = bufLines

	// load window for the first time
	m.buf.AfterLines = [2]string{m.buf.completeBuf[relativePos+1], m.buf.completeBuf[relativePos+2]}
	m.buf.BeforeLines = [2]string{m.buf.completeBuf[relativePos-2], m.buf.completeBuf[relativePos-1]}
	m.buf.lineText = m.buf.completeBuf[relativePos]
	m.buf.rltvBufSelection = int(relativePos)
	m.buf.delIdx = make([]bool, len(m.buf.completeBuf))
	return m, nil
}

type lines struct {
	unselectedItemStyle lipgloss.Style
	selectedItemStyle   lipgloss.Style
	completeBuf         []string
	rltvBufSelection    int // item to highlight and work on
	BeforeLines         [2]string
	lineText            string
	AfterLines          [2]string
	delIdx              []bool
}

func (l lines) String() string {
	var B string
	var A string

	for _, v := range l.BeforeLines {
		if v == "" {
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
	A = strings.TrimSuffix(A, "\n")
	B = strings.TrimSuffix(B, "\n")

	return lipgloss.JoinVertical(
		lipgloss.NoTabConversion,
		l.unselectedItemStyle.Render(B),
		l.selectedItemStyle.Render(l.lineText),
		l.unselectedItemStyle.Render(A),
	)
}

// setWindow changes the loaded lines but not the buffer
//
// startBufInx is the buffer index at which we want to start
func (l lines) setWindow(startBufInx int) (lines, error) {
	maxIndex := len(l.completeBuf) - 1
	if maxIndex < 5 {
		startBufInx = 0
	} else {
		startBufInx = max(0, min(startBufInx, maxIndex-4))
	}

	selected := l.rltvBufSelection
	if selected < startBufInx || selected > startBufInx+4 || selected > maxIndex {
		logrus.Fatal(errorutils.NewReport("selected index out of bounds. Picked: "+strconv.Itoa(selected)+" buffer length: "+strconv.Itoa(maxIndex)+" startBufInx: "+strconv.Itoa(startBufInx), "08MSWPlLllG"))
	}
	relPtr := selected - startBufInx

	var returnable error
	// copture values during a panic
	defer func() {
		if err := recover(); err != nil {
			logrus.Info("recovered from panic: ", err)
			logrus.Infof("[start: %d, bufSelection: %d, relPtr: %d, maxIndex: %d ]\n", startBufInx, l.rltvBufSelection, relPtr, maxIndex)
			logrus.Infof("bounds of after[%d:%d]\n", min(startBufInx+relPtr+2, len(l.completeBuf)), max(startBufInx+relPtr+3, min(startBufInx+5, len(l.completeBuf))))
			returnable = errorutils.NewReport("recovered from panic: "+err.(error).Error(), "7NEKdcE8loi")
		}
	}()
	l.BeforeLines[0] = ""
	l.BeforeLines[1] = ""
	l.AfterLines[0] = ""
	l.AfterLines[1] = ""
	copy(l.BeforeLines[:], l.completeBuf[startBufInx:startBufInx+relPtr])
	l.lineText = l.completeBuf[startBufInx+relPtr]
	copy(l.AfterLines[:], l.completeBuf[min(startBufInx+relPtr+1, len(l.completeBuf)):min(startBufInx+5, len(l.completeBuf))])
	return l, returnable
}

func (l lines) down() (lines, tea.Cmd) {
	var err error
	l.rltvBufSelection++
	if l.rltvBufSelection >= len(l.completeBuf) {
		l.rltvBufSelection = len(l.completeBuf) - 1
	}
	l, err = l.setWindow(l.rltvBufSelection - 2)
	if err != nil {
		return l, tea.Quit
	}
	return l, nil
}

func (l lines) up() (lines, tea.Cmd) {
	var err error
	l.rltvBufSelection--
	if l.rltvBufSelection < 0 {
		l.rltvBufSelection = 0
	}
	l, err = l.setWindow(l.rltvBufSelection - 2)
	if err != nil {
		return l, tea.Quit
	}
	return l, nil
}

type keymap struct {
	Down,
	Up,
	Delete,
	Shuffle,
	Abort,
	Submit key.Binding
}

func defaultKeymap() keymap {
	return keymap{
		Down: key.NewBinding(
			key.WithKeys("down", "j", "ctrl+j", "ctrl+n"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k", "ctrl+k", "ctrl+p"),
		),
		Delete: key.NewBinding(
			key.WithKeys("ctrl+d", "d"),
			key.WithHelp("d", "delete"),
		),
		Shuffle: key.NewBinding(
			key.WithKeys("space", "s"),
			key.WithHelp("space", "shuffle"),
		),
		Abort: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("esc", "abort_w/o_save"),
		),
		Submit: key.NewBinding(
			key.WithKeys("enter", "ctrl+q"),
			key.WithHelp("enter", "submit"),
		),
	}
}
func (k keymap) FullHelp() [][]key.Binding { return nil }

// ShortHelp implements help.KeyMap.
func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(
			key.WithKeys("up", "down"),
			key.WithHelp("↓↑", ""),
		),
		k.Delete,
		k.Shuffle,
		k.Abort,
		k.Submit,
	}
}
