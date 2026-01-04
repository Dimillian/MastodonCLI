package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mastodoncli/internal/mastodon"
	"mastodoncli/internal/output"
)

type timelineItem struct {
	id      string
	title   string
	snippet string
}

func (t timelineItem) Title() string       { return t.title }
func (t timelineItem) Description() string { return t.snippet }
func (t timelineItem) FilterValue() string { return t.title + " " + t.snippet }

type timelineModel struct {
	client   *mastodon.Client
	list     list.Model
	detail   viewport.Model
	statuses []mastodon.Status
	topID    string
	loading  bool
	width    int
	height   int
	selected int
	spinner  spinner.Model
}

type timelineMsg struct {
	statuses []mastodon.Status
	sinceID  string
}

type timelineErrMsg struct {
	err error
}

func Run(client *mastodon.Client) error {
	model := newTimelineModel(client)
	_, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}

func newTimelineModel(client *mastodon.Client) timelineModel {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("86"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("86"))
	delegate.SetHeight(3)

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Home timeline"
	l.SetShowHelp(true)
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(true)
	l.SetShowPagination(true)
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		}
	}
	l.DisableQuitKeybindings()

	vp := viewport.New(0, 0)
	vp.YPosition = 0
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	return timelineModel{
		client:  client,
		list:    l,
		detail:  vp,
		loading: true,
		spinner: sp,
	}
}

func (m timelineModel) Init() tea.Cmd {
	if len(m.statuses) == 0 {
		m.list.SetItems([]list.Item{loadingItem()})
	}
	return tea.Batch(
		m.list.StartSpinner(),
		fetchTimelineCmd(m.client, 40, ""),
		m.spinner.Tick,
	)
}

func (m timelineModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			if m.loading {
				return m, nil
			}
			m.loading = true
			return m, tea.Batch(
				m.list.StartSpinner(),
				fetchTimelineCmd(m.client, 40, m.topID),
				m.spinner.Tick,
			)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		m.renderDetail()
	case timelineMsg:
		m.loading = false
		m.list.StopSpinner()
		m.list.Title = "Home timeline"
		if msg.sinceID != "" {
			m.prependStatuses(msg.statuses)
			if len(msg.statuses) == 0 {
				return m, m.list.NewStatusMessage("No new statuses.")
			}
			m.renderDetail()
			return m, m.list.NewStatusMessage(fmt.Sprintf("Fetched %d new statuses.", len(msg.statuses)))
		}
		m.setStatuses(msg.statuses)
		m.renderDetail()
		if len(msg.statuses) == 0 {
			return m, m.list.NewStatusMessage("No statuses returned.")
		}
		return m, m.list.NewStatusMessage(fmt.Sprintf("Loaded %d statuses.", len(msg.statuses)))
	case timelineErrMsg:
		m.loading = false
		m.list.StopSpinner()
		m.list.Title = "Home timeline"
		return m, m.list.NewStatusMessage(fmt.Sprintf("Error: %v", msg.err))
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.loading {
			m.list.Title = fmt.Sprintf("%s Home timeline", m.spinner.View())
			m.renderDetail()
			return m, cmd
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.list.Index() != m.selected {
		m.selected = m.list.Index()
		m.renderDetail()
	}

	m.detail, _ = m.detail.Update(msg)
	return m, cmd
}

func (m timelineModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	leftWidth := max(30, m.width/2)
	rightWidth := m.width - leftWidth - 1
	if rightWidth < 20 {
		return m.list.View()
	}

	left := lipgloss.NewStyle().Width(leftWidth).Height(m.height).Render(m.list.View())
	right := lipgloss.NewStyle().Width(rightWidth).Height(m.height).Render(m.detail.View())
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("│")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right)
}

func (m *timelineModel) resize() {
	if m.width == 0 || m.height == 0 {
		return
	}

	leftWidth := max(30, m.width/2)
	rightWidth := m.width - leftWidth - 1
	listHeight := max(5, m.height-2)

	m.list.SetSize(leftWidth, listHeight)
	m.detail.Width = rightWidth
	m.detail.Height = max(5, m.height-2)
}

func (m *timelineModel) setStatuses(statuses []mastodon.Status) {
	m.statuses = statuses
	items := make([]list.Item, 0, max(1, len(statuses)))
	if len(statuses) == 0 {
		items = append(items, emptyItem())
	} else {
		for _, item := range statuses {
			items = append(items, statusToItem(item, m.list.Width()))
		}
	}
	m.list.SetItems(items)
	if len(statuses) > 0 {
		m.topID = statuses[0].ID
	}
}

func (m *timelineModel) prependStatuses(statuses []mastodon.Status) {
	if len(statuses) == 0 {
		return
	}

	m.statuses = append(statuses, m.statuses...)
	items := make([]list.Item, 0, max(1, len(m.statuses)))
	for _, item := range m.statuses {
		items = append(items, statusToItem(item, m.list.Width()))
	}
	m.list.SetItems(items)
	m.topID = statuses[0].ID
}

func (m *timelineModel) renderDetail() {
	if m.detail.Width == 0 {
		return
	}
	if len(m.statuses) == 0 {
		if m.loading {
			m.detail.SetContent(fmt.Sprintf("%s Loading timeline...", m.spinner.View()))
		} else {
			m.detail.SetContent("No status selected.")
		}
		return
	}

	index := m.list.Index()
	if index < 0 || index >= len(m.statuses) {
		index = 0
	}

	m.detail.SetContent(renderStatusDetail(m.statuses[index], m.detail.Width))
}

func statusToItem(item mastodon.Status, width int) timelineItem {
	display := &item
	boostedBy := ""
	if item.Reblog != nil {
		boostedBy = fmt.Sprintf(" · boosted by @%s", item.Account.Acct)
		display = item.Reblog
	}

	name := strings.TrimSpace(output.StripHTML(display.Account.DisplayName))
	author := fmt.Sprintf("@%s", display.Account.Acct)
	if name != "" && name != display.Account.Acct {
		author = fmt.Sprintf("%s (@%s)", name, display.Account.Acct)
	}

	title := fmt.Sprintf("%s%s · %s", author, boostedBy, display.CreatedAt)
	snippet := output.WrapText(output.StripHTML(display.Content), max(20, width-6))
	snippet = truncateLines(snippet, 2)
	if snippet == "" {
		snippet = "(no text)"
	}

	return timelineItem{
		id:      display.ID,
		title:   title,
		snippet: snippet,
	}
}

func loadingItem() timelineItem {
	return timelineItem{
		title:   "Loading timeline...",
		snippet: "Fetching latest statuses...",
	}
}

func emptyItem() timelineItem {
	return timelineItem{
		title:   "No statuses",
		snippet: "Your home timeline is empty.",
	}
}

func renderStatusDetail(item mastodon.Status, width int) string {
	display := &item
	boostedBy := ""
	if item.Reblog != nil {
		boostedBy = fmt.Sprintf("@%s", item.Account.Acct)
		display = item.Reblog
	}

	name := strings.TrimSpace(output.StripHTML(display.Account.DisplayName))
	author := fmt.Sprintf("@%s", display.Account.Acct)
	if name != "" && name != display.Account.Acct {
		author = fmt.Sprintf("%s (@%s)", name, display.Account.Acct)
	}

	wrapWidth := max(20, width-2)
	separator := strings.Repeat("-", width)

	var builder strings.Builder
	builder.WriteString(separator)
	builder.WriteString("\n")
	builder.WriteString(authorStyle.Render("Author:"))
	builder.WriteString(" ")
	builder.WriteString(author)
	builder.WriteString("\n")
	builder.WriteString(timeStyle.Render("Time:"))
	builder.WriteString("   ")
	builder.WriteString(display.CreatedAt)
	builder.WriteString("\n")
	if boostedBy != "" {
		builder.WriteString(mutedStyle.Render("Boost:"))
		builder.WriteString("  ")
		builder.WriteString(boostedBy)
		builder.WriteString("\n")
	}
	builder.WriteString("Text:\n")
	text := output.WrapText(output.StripHTML(display.Content), wrapWidth)
	if text == "" {
		text = "(no text)"
	}
	builder.WriteString(text)

	return builder.String()
}

func truncateLines(text string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}

	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return text
	}

	return strings.Join(lines[:maxLines], "\n") + "…"
}

func fetchTimelineCmd(client *mastodon.Client, limit int, sinceID string) tea.Cmd {
	return func() tea.Msg {
		statuses, err := client.HomeTimelinePage(limit, sinceID, "")
		if err != nil {
			return timelineErrMsg{err: err}
		}
		return timelineMsg{statuses: statuses, sinceID: sinceID}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var (
	authorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	timeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)
