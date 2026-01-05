package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mastodoncli/internal/mastodon"
	"mastodoncli/internal/output"
	"mastodoncli/internal/ui/components"
)

type notificationsView struct {
	list          list.Model
	detail        viewport.Model
	notifications []mastodon.GroupedNotification
	loading       bool
	selected      int
}

type notificationsMsg struct {
	notifications []mastodon.GroupedNotification
}

type notificationItem struct {
	title   string
	snippet string
}

func (n notificationItem) Title() string       { return n.title }
func (n notificationItem) Description() string { return n.snippet }
func (n notificationItem) FilterValue() string { return n.title + " " + n.snippet }

func newNotificationsView(title string) *notificationsView {
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

	return &notificationsView{
		list:    l,
		detail:  vp,
		loading: true,
	}
}

func (m *model) ensureNotificationsLoaded() tea.Cmd {
	view := m.notificationsView
	if !view.loading && len(view.notifications) > 0 {
		return nil
	}
	view.loading = true
	view.list.SetItems([]list.Item{loadingItem()})
	view.list.StartSpinner()
	return tea.Batch(
		fetchNotificationsCmd(m.client),
		m.spinner.Tick,
	)
}

func fetchNotificationsCmd(client *mastodon.Client) tea.Cmd {
	return func() tea.Msg {
		notifications, err := client.GroupedNotifications(40)
		if err != nil {
			return feedErrMsg{tab: tabNotifications, err: err}
		}
		return notificationsMsg{notifications: notifications}
	}
}

func (m *model) setNotifications(view *notificationsView, notifications []mastodon.GroupedNotification) {
	view.notifications = notifications
	items := make([]list.Item, 0, components.Max(1, len(notifications)))
	if len(notifications) == 0 {
		items = append(items, emptyItem())
	} else {
		for _, item := range notifications {
			items = append(items, notificationToItem(item, view.list.Width()))
		}
	}
	view.list.SetItems(items)
}

func (m *model) renderNotificationsDetail(view *notificationsView) {
	if view.detail.Width == 0 {
		return
	}
	if len(view.notifications) == 0 {
		if view.loading {
			view.detail.SetContent(fmt.Sprintf("%s Loading notifications...", m.spinner.View()))
		} else {
			view.detail.SetContent("No notification selected.")
		}
		return
	}

	index := view.list.Index()
	if index < 0 || index >= len(view.notifications) {
		index = 0
	}

	view.detail.SetContent(renderNotificationDetail(view.notifications[index], view.detail.Width))
}

func notificationToItem(item mastodon.GroupedNotification, width int) notificationItem {
	author := notificationAccountsLabel(item.Accounts)
	title := fmt.Sprintf("%s (%d) · %s · %s", notificationTypeLabel(item.Type), item.Count, author, notificationLatestLabel(item))
	snippet := ""
	if item.Status != nil {
		snippet = output.WrapText(output.StripHTML(item.Status.Content), components.Max(20, width-6))
		snippet = components.TruncateLines(snippet, 2)
	}
	if snippet == "" {
		snippet = "(no text)"
	}

	return notificationItem{
		title:   title,
		snippet: snippet,
	}
}

func renderNotificationDetail(item mastodon.GroupedNotification, width int) string {
	author := notificationAccountsLabel(item.Accounts)
	wrapWidth := components.Max(20, width-2)
	separator := strings.Repeat("-", width)

	var builder strings.Builder
	builder.WriteString(separator)
	builder.WriteString("\n")
	builder.WriteString(components.AuthorStyle.Render("Type:"))
	builder.WriteString(" ")
	builder.WriteString(notificationTypeLabel(item.Type))
	builder.WriteString("\n")
	builder.WriteString(components.AuthorStyle.Render("From:"))
	builder.WriteString(" ")
	builder.WriteString(author)
	builder.WriteString("\n")
	builder.WriteString(components.TimeStyle.Render("Time:"))
	builder.WriteString("   ")
	builder.WriteString(notificationLatestLabel(item))
	builder.WriteString("\n")
	builder.WriteString(components.MutedStyle.Render("Count:"))
	builder.WriteString("  ")
	builder.WriteString(fmt.Sprintf("%d", item.Count))
	builder.WriteString("\n")

	if item.Status != nil {
		builder.WriteString("Text:\n")
		text := output.WrapText(output.StripHTML(item.Status.Content), wrapWidth)
		if text == "" {
			text = "(no text)"
		}
		builder.WriteString(text)
	}

	return builder.String()
}

func notificationAccountsLabel(accounts []mastodon.Account) string {
	if len(accounts) == 0 {
		return "Unknown"
	}
	if len(accounts) == 1 {
		return formatAccount(accounts[0])
	}
	first := formatAccount(accounts[0])
	return fmt.Sprintf("%s +%d", first, len(accounts)-1)
}

func notificationLatestLabel(item mastodon.GroupedNotification) string {
	if item.LatestAt == "" {
		return "Unknown"
	}
	return item.LatestAt
}

func formatAccount(account mastodon.Account) string {
	name := strings.TrimSpace(output.StripHTML(account.DisplayName))
	if name != "" && name != account.Acct {
		return fmt.Sprintf("%s (@%s)", name, account.Acct)
	}
	return fmt.Sprintf("@%s", account.Acct)
}

func notificationTypeLabel(value string) string {
	switch value {
	case "mention":
		return "Mention"
	case "status":
		return "Status"
	case "reblog":
		return "Boost"
	case "favourite":
		return "Favorite"
	case "follow":
		return "Follow"
	case "follow_request":
		return "Follow request"
	case "poll":
		return "Poll"
	case "update":
		return "Update"
	case "admin.sign_up":
		return "Sign up"
	case "admin.report":
		return "Report"
	default:
		return value
	}
}
