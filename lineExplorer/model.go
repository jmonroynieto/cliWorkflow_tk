package main

import (
	"os"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func New(file *os.File, idx index) Model {
	h := help.New()
	h.ShortSeparator = " \x1b[94m|\x1b[0m "
	selectioncolor := lipgloss.AdaptiveColor{Light: "#000000", Dark: "#D6BA7C"}

	return Model{
		file:         file,
		idx:          idx,
		shouldDelete: make(map[uint32]struct{}),
		keymap:       defaultKeymap(),
		buf: lines{
			unselectedItemStyle: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#37282f", Dark: "#4F6367"}),
			selectedItemStyle:   lipgloss.NewStyle().Foreground(selectioncolor),
			AfterLines:          make([]string, 2),
			BeforeLines:         make([]string, 2),
			cursorcolor:         selectioncolor,
		},
		help:            h,
		shouldOverwrite: true, //default assumption
		titleStyle:      lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#D6BA7C", Dark: "#131727"}).Background(selectioncolor),
	}
}

type Model struct {
	keymap          keymap
	help            help.Model
	file            *os.File
	idx             index
	currentLine     uint32
	buf             lines //max renders 2 + 1 + 2 but keeps 30 in memory
	shouldDelete    map[uint32]struct{}
	shouldOverwrite bool
	titleStyle      lipgloss.Style
}

func (m Model) Init() tea.Cmd { return func() tea.Msg { return shuffleMsg{} } }

type shuffleMsg struct{}
type closeMsg struct{ err error }
type noopMsg struct{} //do nothing

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	//handle messages
	case tea.WindowSizeMsg:
		return m, nil
	case shuffleMsg:
		return m.shuffleAndSetLines()
	case noopMsg:
		// this enables the user to see the loading screen and the termination sequence to be able to delete only one line from the scrollback buffer/tea window.
		time.Sleep(100 * time.Millisecond)
		return m, nil
	case closeMsg:
		m.buf.completeBuf = make([]string, 0)
		m.buf.bufSelectIndex = 0
		return m, tea.Sequence(nil, func() tea.Msg { return noopMsg{} }, tea.Quit)
	case tea.KeyMsg:
		// handle key messages
		km := m.keymap
		errCMD := tea.Cmd(nil)
		switch {
		case key.Matches(msg, km.Down):
			m.buf, errCMD = m.buf.down()
			return m, errCMD
		case key.Matches(msg, km.Up):
			m.buf, errCMD = m.buf.up()
			return m, errCMD
		case key.Matches(msg, km.Delete):
			if m.buf.delIdx[m.buf.bufSelectIndex] {
				m.buf.delIdx[m.buf.bufSelectIndex] = false
				delete(m.shouldDelete, uint32(m.buf.selectedFileIndex()))
			} else {
				m.buf.delIdx[m.buf.bufSelectIndex] = true
				m.shouldDelete[uint32(m.buf.selectedFileIndex())] = struct{}{}
			}
			m.buf.gutterLines = m.buf.updateGutter()
			return m, nil
		case key.Matches(msg, km.Shuffle):
			return m, func() tea.Msg { return shuffleMsg{} }
		case key.Matches(msg, km.Abort):
			m.shouldOverwrite = false

			return m, func() tea.Msg { return closeMsg{} }
		case key.Matches(msg, km.Submit):
			return m, func() tea.Msg { return closeMsg{} }
		default:
			// noop
		}
	}
	return m, nil
}

func (m Model) View() string {
	if len(m.buf.completeBuf) == 0 {
		return "loading..."
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title(m.currentLine, &(m.titleStyle), len(m.idx)),
		m.buf.String(),
		"",
		m.help.View(m.keymap),
	)
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
			key.WithKeys("ctrl+d", "d", "x"),
			key.WithHelp("d", "delete"),
		),
		Shuffle: key.NewBinding(
			key.WithKeys(" ", "s"),
			key.WithHelp("space", "shuffle"),
		),
		Abort: key.NewBinding(
			key.WithKeys("esc", "ctrl+c", "q"),
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
