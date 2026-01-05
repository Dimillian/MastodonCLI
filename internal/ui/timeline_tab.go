package ui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mastodoncli/internal/mastodon"
	"mastodoncli/internal/ui/components"
)

type timelineMode int

const (
	modeHome timelineMode = iota
	modeLocal
	modeFederated
	modeTrending
)

type timelineMsg struct {
	mode     timelineMode
	statuses []mastodon.Status
	sinceID  string
}

func (m model) renderTimelineModes() string {
	labels := []string{"Home", "Local", "Federated", "Trending"}
	modes := []timelineMode{modeHome, modeLocal, modeFederated, modeTrending}
	var parts []string
	for i, label := range labels {
		style := components.ModeStyle
		if m.activeTimeline == modes[i] {
			style = components.ModeActiveStyle
		}
		parts = append(parts, components.RenderTabLabel(label, style))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m *model) switchTimelineMode(mode timelineMode) (tea.Model, tea.Cmd) {
	if m.activeTimeline == mode {
		return m, nil
	}
	m.activeTimeline = mode
	m.resizeAll()
	m.renderCurrentDetail()
	return m, m.ensureTimelineLoaded()
}

func (m *model) ensureTimelineLoaded() tea.Cmd {
	view := m.timelineView()
	if !view.loading && len(view.statuses) > 0 {
		return nil
	}
	view.loading = true
	view.list.SetItems([]list.Item{loadingItem()})
	view.list.StartSpinner()
	return tea.Batch(
		fetchTimelineCmd(m.client, m.activeTimeline, ""),
		m.spinner.Tick,
	)
}

func fetchTimelineCmd(client *mastodon.Client, mode timelineMode, sinceID string) tea.Cmd {
	return func() tea.Msg {
		var statuses []mastodon.Status
		var err error
		switch mode {
		case modeHome:
			statuses, err = client.HomeTimelinePage(40, sinceID, "")
		case modeLocal:
			statuses, err = client.PublicTimelinePage(40, true, false, sinceID, "")
		case modeFederated:
			statuses, err = client.PublicTimelinePage(40, false, false, sinceID, "")
		case modeTrending:
			statuses, err = client.TrendingStatuses(40)
		default:
			statuses = nil
		}
		if err != nil {
			return feedErrMsg{tab: tabTimeline, mode: mode, err: err}
		}
		return timelineMsg{mode: mode, statuses: statuses, sinceID: sinceID}
	}
}
