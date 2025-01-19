package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	filepicker      filepicker.Model
	selectedFile    string
	hurlOutput      string
	quitting        bool
	filePickerError error
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
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}
	case clearErrorMsg:
		m.filePickerError = nil
	}

	var cmd tea.Cmd
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

	return m, cmd
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

var borderStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())

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
	return borderStyle.Render(s.String())
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
	return borderStyle.Render(s.String())
}

func (m model) hurlView() string {
	var s strings.Builder
	s.WriteString("hurl Output:")
	s.WriteString("\n")
	s.WriteString("\n")
	s.WriteString(string(m.hurlOutput))
	return borderStyle.Render(s.String())
}

func main() {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".hurl"}
	fp.CurrentDirectory = "/home/yym/SSD-1TB/coding/repos/hurl"
	fp.AutoHeight = false
	fp.Height = 10
	m := model{
		filepicker: fp,
	}
	tm, _ := tea.NewProgram(&m).Run()
	mm := tm.(model)
	fmt.Println("\n  You selected: " + m.filepicker.Styles.Selected.Render(mm.selectedFile) + "\n")
}
