package main

import (
	"os"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
			AfterLines:          make([]string, 2),
			BeforeLines:         make([]string, 2),
		},
		help:            h,
		shouldOverwrite: true, //default assumption
		askingForInput:  false,
	}
}

type Model struct {
	currentLine     uint32
	file            *os.File
	idx             index
	shouldDelete    map[uint32]struct{}
	buf             lines //max renders 2 + 1 + 2 but keeps 30 in memory
	keymap          keymap
	help            help.Model
	shouldOverwrite bool
	askingForInput  bool
}

func (m Model) Init() tea.Cmd { return func() tea.Msg { return shuffleMsg{} } }

type shuffleMsg struct{}
type closeMsg struct{ err error }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, nil
	case shuffleMsg:
		return m.shuffleAndSetLines()
	case closeMsg:
		m.buf.reset()
		return m, nil
	case tea.KeyMsg:
		km := m.keymap
		errCMD := tea.Cmd(nil)
		switch {
		case m.askingForInput:
			switch msg.String() {
			case "y":
				m.shouldOverwrite = true
			case "n":
				m.shouldOverwrite = false
			}
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
			if msg.String() == "ctrl+c" {
				return New(nil, nil), tea.Quit
			}
			return New(nil, nil), func() tea.Msg { return closeMsg{} }
		case key.Matches(msg, km.Submit):
			return m, func() tea.Msg { return closeMsg{} }
		default:
		}
	}
	return m, nil
}

func (m Model) View() string {
	if len(m.buf.completeBuf) == 0 {
		return "loading..."
	}
	if m.askingForInput {
		return "Original file changed while selecting lines to delete, overwrite? (y/n)"
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title(m.currentLine),
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
			key.WithKeys("space", "s"),
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
