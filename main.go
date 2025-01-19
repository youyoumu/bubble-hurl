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
	dump            io.Writer
	filepicker      filepicker.Model
	selectedFile    string
	hurlOutput      string
	quitting        bool
	filePickerError error
	hurlViewport    viewport.Model
	ready           bool
	activeWindow    int
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
			// Since this program is using the full size of the viewport we
			// need to wait until we've received the window dimensions before
			// we can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we wait for them
			// here.
			m.hurlViewport = viewport.New(msg.Width-2, 10)
			// m.hurlViewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.hurlViewport.Width = msg.Width - 2
			m.hurlViewport.Height = 10
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
		m.filePickerError = nil
	}

	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	m.filepicker, cmd = m.filepicker.Update(msg)

	// Did the user select a file?
	if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
		// Get the path of the selected file.
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

	// Did the user select a disabled file?
	// This is only necessary to display an error to the user.
	if didSelect, path := m.filepicker.DidSelectDisabledFile(msg); didSelect {
		// Let's clear the selectedFile and display an error.
		m.filePickerError = errors.New(path + " is not valid.")
		m.selectedFile = ""
		return m, tea.Batch(cmd, clearErrorAfter(2*time.Second))
	}

	// m.hurlViewport, cmd = m.hurlViewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	var s strings.Builder
	s.WriteString(m.filePickerView())
	s.WriteString("\n")
	s.WriteString(m.catView())
	s.WriteString("\n")
	s.WriteString(m.hurlView())

	return s.String()
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
	if m.filePickerError != nil {
		s.WriteString(m.filepicker.Styles.DisabledFile.Render(m.filePickerError.Error()))
	} else if m.selectedFile == "" {
		s.WriteString("Pick a file:")
	} else {
		s.WriteString("Selected file: " + m.filepicker.Styles.Selected.Render(m.selectedFile))
	}
	s.WriteString("\n")
	s.WriteString(m.filepicker.View())
	return borderStyle(m.activeWindow == 1).Render(s.String())
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
	return borderStyle(m.activeWindow == 2).Render(s.String())
}

func (m model) hurlView() string {
	var s strings.Builder
	s.WriteString("hurl Output:")
	s.WriteString("\n")
	s.WriteString("\n")
	s.WriteString(string(m.hurlOutput))
	m.hurlViewport.SetContent(s.String())
	return borderStyle(m.activeWindow == 3).Render(m.hurlViewport.View())
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
	fp.Height = 10

	m := model{
		activeWindow: 1,
		dump:         dump,
		filepicker:   fp,
	}
	if _, err := tea.NewProgram(&m).Run(); err != nil {
		fmt.Println("could not start program:", err)
		os.Exit(1)
	}
}
