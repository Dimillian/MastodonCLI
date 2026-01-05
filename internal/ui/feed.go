package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"mastodoncli/internal/mastodon"
	"mastodoncli/internal/output"
	"mastodoncli/internal/ui/components"
)

type timelineItem struct {
	id      string
	title   string
	snippet string
}

func (t timelineItem) Title() string       { return t.title }
func (t timelineItem) Description() string { return t.snippet }
func (t timelineItem) FilterValue() string { return t.title + " " + t.snippet }

type feedView struct {
	list     list.Model
	detail   viewport.Model
	statuses []mastodon.Status
	topID    string
	loading  bool
	selected int
}

func newFeedView(title string) *feedView {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("86"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("86"))
	delegate.SetHeight(3)

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = title
	l.SetShowHelp(true)
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(true)
	l.SetShowPagination(true)
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
			key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev tab")),
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		}
	}
	l.DisableQuitKeybindings()

	vp := viewport.New(0, 0)
	vp.YPosition = 0

	return &feedView{
		list:    l,
		detail:  vp,
		loading: true,
	}
}

func (m *model) setStatuses(view *feedView, statuses []mastodon.Status) {
	view.statuses = statuses
	items := make([]list.Item, 0, components.Max(1, len(statuses)))
	if len(statuses) == 0 {
		items = append(items, emptyItem())
	} else {
		for _, item := range statuses {
			items = append(items, statusToItem(item, view.list.Width()))
		}
	}
	view.list.SetItems(items)
	if len(statuses) > 0 {
		view.topID = statuses[0].ID
	}
}

func (m *model) prependStatuses(view *feedView, statuses []mastodon.Status) {
	if len(statuses) == 0 {
		return
	}

	view.statuses = append(statuses, view.statuses...)
	items := make([]list.Item, 0, components.Max(1, len(view.statuses)))
	for _, item := range view.statuses {
		items = append(items, statusToItem(item, view.list.Width()))
	}
	view.list.SetItems(items)
	view.topID = statuses[0].ID
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
	snippet := output.WrapText(output.StripHTML(display.Content), components.Max(20, width-6))
	snippet = components.TruncateLines(snippet, 2)
	if snippet == "" {
		snippet = "(no text)"
	}

	return timelineItem{
		id:      display.ID,
		title:   title,
		snippet: snippet,
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

	wrapWidth := components.Max(20, width-2)
	separator := strings.Repeat("-", width)

	var builder strings.Builder
	builder.WriteString(separator)
	builder.WriteString("\n")
	builder.WriteString(components.AuthorStyle.Render("Author:"))
	builder.WriteString(" ")
	builder.WriteString(author)
	builder.WriteString("\n")
	builder.WriteString(components.TimeStyle.Render("Time:"))
	builder.WriteString("   ")
	builder.WriteString(display.CreatedAt)
	builder.WriteString("\n")
	if boostedBy != "" {
		builder.WriteString(components.MutedStyle.Render("Boost:"))
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

func loadingItem() timelineItem {
	return timelineItem{
		title:   "Loading timeline...",
		snippet: "Fetching latest statuses...",
	}
}

func emptyItem() timelineItem {
	return timelineItem{
		title:   "No statuses",
		snippet: "Nothing to show here yet.",
	}
}
