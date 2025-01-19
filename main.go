package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kortschak/utter"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	dump               io.Writer
	filepicker         filepicker.Model
	selectedFile       string
	hurlOutput         string
	quitting           bool
	filepickerError    error
	filepickerViewport viewport.Model
	catViewport        viewport.Model
	hurlViewport       viewport.Model
	ready              bool
	activeWindow       int
}

type clearErrorMsg struct{}

func clearErrorAfter(t time.Duration) tea.Cmd {
	return tea.Tick(t, func(_ time.Time) tea.Msg {
		return clearErrorMsg{}
	})
}

func (m model) Init() tea.Cmd {
	return m.filepicker.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.dump != nil {
		utter.Fdump(m.dump, msg)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.ready {

			m.filepicker.Height = 10
			m.filepickerViewport = viewport.New((msg.Width/2)-2, (msg.Height/2)-2)
			m.catViewport = viewport.New((msg.Width/2)-2, (msg.Height/2)-2)
			m.hurlViewport = viewport.New((msg.Width/2)-2, (msg.Height)-2)
			m.ready = true
		} else {

			m.filepickerViewport.Width = (msg.Width / 2) - 2
			m.filepickerViewport.Height = (msg.Height / 2) - 2
			m.catViewport.Width = (msg.Width / 2) - 2
			m.catViewport.Height = (msg.Height / 2) - 2
			m.hurlViewport.Width = (msg.Width / 2) - 2
			m.hurlViewport.Height = (msg.Height) - 2
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "1":
			m.activeWindow = 1
		case "2":
			m.activeWindow = 2
		case "3":
			m.activeWindow = 3
		}
	case clearErrorMsg:
		m.filepickerError = nil
	}

	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch m.activeWindow {
	case 1:
		m.filepicker, cmd = m.filepicker.Update(msg)
		cmds = append(cmds, cmd)

		if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
			m.selectedFile = path
			if m.selectedFile == path {
				hurlOutput, err := exec.Command("hurl", path, "--variables-file", "/home/yym/SSD-1TB/coding/repos/hurl/hurl.env").CombinedOutput()
				if err == nil {
					jqOutput, jqErr := exec.Command("jq", "--color-output", "--null-input", "--jsonargs", string(hurlOutput)).CombinedOutput()
					if jqErr == nil {
						m.hurlOutput = string(jqOutput)
					} else {
						m.hurlOutput = jqErr.Error()
					}
				} else {
					m.hurlOutput = err.Error() + "\n" + string(hurlOutput)
				}
			}
		}

		if didSelectDisabled, path := m.filepicker.DidSelectDisabledFile(msg); didSelectDisabled {
			m.filepickerError = errors.New(path + " is not valid.")
			m.selectedFile = ""
			cmds = append(cmds, clearErrorAfter(2*time.Second))
		}
	case 2:
		m.catViewport, cmd = m.catViewport.Update(msg)
		cmds = append(cmds, cmd)
	case 3:
		m.hurlViewport, cmd = m.hurlViewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.filepickerViewport.Style = borderStyle(m.activeWindow == 1)
	m.filepickerViewport.SetContent(m.filePickerView())
	m.catViewport.Style = borderStyle(m.activeWindow == 2)
	m.catViewport.SetContent(m.catView())
	m.hurlViewport.Style = borderStyle(m.activeWindow == 3)
	m.hurlViewport.SetContent(m.hurlView())

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	var left strings.Builder
	left.WriteString(m.filepickerViewport.View())
	left.WriteString("\n")
	left.WriteString(m.catViewport.View())

	var right strings.Builder
	right.WriteString(m.hurlViewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Center, left.String(), right.String())
}

func borderStyle(active bool) lipgloss.Style {
	style := lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())
	if !active {
		return style
	} else {
		return style.BorderForeground(lipgloss.Color("011"))
	}
}

func (m model) filePickerView() string {
	var s strings.Builder
	if m.filepickerError != nil {
		s.WriteString(m.filepicker.Styles.DisabledFile.Render(m.filepickerError.Error()))
	} else if m.selectedFile == "" {
		s.WriteString("Pick a file:")
	} else {
		s.WriteString("Selected file: " + m.filepicker.Styles.Selected.Render(m.selectedFile))
	}
	s.WriteString("\n")
	s.WriteString(m.filepicker.View())
	return s.String()
}

func (m model) catView() string {
	var s strings.Builder
	s.WriteString("cat Output:")
	s.WriteString("\n")
	s.WriteString("\n")
	if m.selectedFile != "" {
		out, _ := exec.Command("cat", m.selectedFile).CombinedOutput()
		s.WriteString(string(out))
	}
	return s.String()
}

func (m model) hurlView() string {
	var s strings.Builder
	s.WriteString("hurl Output:")
	s.WriteString("\n")
	s.WriteString("\n")
	s.WriteString(string(m.hurlOutput))
	return s.String()
}

func main() {
	var dump *os.File
	if _, ok := os.LookupEnv("DEBUG"); ok {
		var err error
		dump, err = os.OpenFile("messages.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			os.Exit(1)
		}
	}

	fp := filepicker.New()
	fp.AllowedTypes = []string{".hurl"}
	fp.CurrentDirectory = "/home/yym/SSD-1TB/coding/repos/hurl"
	fp.AutoHeight = false

	m := model{
		activeWindow: 1,
		dump:         dump,
		filepicker:   fp,
	}
	if _, err := tea.NewProgram(&m, tea.WithMouseCellMotion()).Run(); err != nil {
		fmt.Println("could not start program:", err)
		os.Exit(1)
	}
}
