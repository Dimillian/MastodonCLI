package components

import "github.com/charmbracelet/lipgloss"

var (
	HeaderStyle     = lipgloss.NewStyle().PaddingLeft(1)
	TabStyle        = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("241"))
	TabActiveStyle  = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("86")).Bold(true)
	ModeStyle       = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("245"))
	ModeActiveStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("86")).Bold(true)
	AuthorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	TimeStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	MutedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func RenderTabLabel(label string, style lipgloss.Style) string {
	if label == "" {
		return style.Render(label)
	}

	runes := []rune(label)
	first := lipgloss.NewStyle().Underline(true).Render(string(runes[0]))
	if len(runes) == 1 {
		return style.Render(first)
	}
	return style.Render(first + string(runes[1:]))
}
