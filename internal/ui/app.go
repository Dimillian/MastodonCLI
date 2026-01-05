package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mastodoncli/internal/mastodon"
	"mastodoncli/internal/ui/components"
)

type topTab int

const (
	tabTimeline topTab = iota
	tabSearch
	tabProfile
	tabNotifications
)

type model struct {
	client            *mastodon.Client
	activeTab         topTab
	activeTimeline    timelineMode
	timelineViews     map[timelineMode]*feedView
	profileView       *feedView
	notificationsView *notificationsView
	searchView        searchView
	profileAccountID  string
	spinner           spinner.Model
	width             int
	height            int
}

type feedErrMsg struct {
	tab  topTab
	mode timelineMode
	err  error
}

func Run(client *mastodon.Client) error {
	m := newModel(client)
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func newModel(client *mastodon.Client) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	timelineViews := map[timelineMode]*feedView{
		modeHome:      newFeedView("Home timeline"),
		modeLocal:     newFeedView("Local timeline"),
		modeFederated: newFeedView("Federated timeline"),
		modeTrending:  newFeedView("Trending"),
	}

	profile := newFeedView("Profile")
	notifications := newNotificationsView("Notifications")
	search := newSearchView()

	return model{
		client:            client,
		activeTab:         tabTimeline,
		activeTimeline:    modeHome,
		timelineViews:     timelineViews,
		profileView:       profile,
		notificationsView: notifications,
		searchView:        search,
		spinner:           sp,
	}
}

func (m model) Init() tea.Cmd {
	m.timelineView().list.SetItems([]list.Item{loadingItem()})
	m.timelineView().list.StartSpinner()
	return tea.Batch(
		fetchTimelineCmd(m.client, modeHome, ""),
		m.spinner.Tick,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeAll()
		m.renderCurrentDetail()
		m.renderSearch()
	case timelineMsg:
		view := m.timelineViews[msg.mode]
		view.loading = false
		view.list.StopSpinner()
		if msg.sinceID != "" {
			m.prependStatuses(view, msg.statuses)
			m.renderCurrentDetail()
			if len(msg.statuses) == 0 {
				return m, view.list.NewStatusMessage("No new statuses.")
			}
			return m, view.list.NewStatusMessage(fmt.Sprintf("Fetched %d new statuses.", len(msg.statuses)))
		}
		m.setStatuses(view, msg.statuses)
		m.renderCurrentDetail()
		if len(msg.statuses) == 0 {
			return m, view.list.NewStatusMessage("No statuses returned.")
		}
		return m, view.list.NewStatusMessage(fmt.Sprintf("Loaded %d statuses.", len(msg.statuses)))
	case profileMsg:
		view := m.profileView
		view.loading = false
		view.list.StopSpinner()
		if msg.accountID != "" {
			m.profileAccountID = msg.accountID
		}
		m.setStatuses(view, msg.statuses)
		m.renderCurrentDetail()
		if len(msg.statuses) == 0 {
			return m, view.list.NewStatusMessage("No statuses returned.")
		}
		return m, view.list.NewStatusMessage(fmt.Sprintf("Loaded %d statuses.", len(msg.statuses)))
	case notificationsMsg:
		view := m.notificationsView
		view.loading = false
		view.list.StopSpinner()
		m.setNotifications(view, msg.notifications)
		m.renderCurrentDetail()
		if len(msg.notifications) == 0 {
			return m, view.list.NewStatusMessage("No notifications returned.")
		}
		return m, view.list.NewStatusMessage(fmt.Sprintf("Loaded %d notifications.", len(msg.notifications)))
	case feedErrMsg:
		if msg.tab == tabTimeline {
			view := m.timelineViews[msg.mode]
			view.loading = false
			view.list.StopSpinner()
			return m, view.list.NewStatusMessage(fmt.Sprintf("Error: %v", msg.err))
		}
		if msg.tab == tabProfile {
			view := m.profileView
			view.loading = false
			view.list.StopSpinner()
			return m, view.list.NewStatusMessage(fmt.Sprintf("Error: %v", msg.err))
		}
		if msg.tab == tabNotifications {
			view := m.notificationsView
			view.loading = false
			view.list.StopSpinner()
			return m, view.list.NewStatusMessage(fmt.Sprintf("Error: %v", msg.err))
		}
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.isLoading() {
			m.renderCurrentDetail()
			return m, cmd
		}
	}

	return m.updateActiveView(msg)
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	header := m.renderHeader()
	content := m.renderContent()

	return header + "\n" + content
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab":
		m.activeTab = (m.activeTab + 1) % 4
		m.resizeAll()
		m.renderCurrentDetail()
		m.renderSearch()
		return m, m.ensureTabLoaded()
	case "shift+tab":
		m.activeTab = (m.activeTab + 3) % 4
		m.resizeAll()
		m.renderCurrentDetail()
		m.renderSearch()
		return m, m.ensureTabLoaded()
	case "t":
		m.activeTab = tabTimeline
		m.resizeAll()
		m.renderCurrentDetail()
		m.renderSearch()
		return m, m.ensureTabLoaded()
	case "s":
		m.activeTab = tabSearch
		m.resizeAll()
		m.renderSearch()
		return m, nil
	case "p":
		m.activeTab = tabProfile
		m.resizeAll()
		m.renderCurrentDetail()
		m.renderSearch()
		return m, m.ensureTabLoaded()
	case "n":
		m.activeTab = tabNotifications
		m.resizeAll()
		m.renderCurrentDetail()
		m.renderSearch()
		return m, m.ensureTabLoaded()
	case "h":
		if m.activeTab == tabTimeline {
			return m.switchTimelineMode(modeHome)
		}
	case "l":
		if m.activeTab == tabTimeline {
			return m.switchTimelineMode(modeLocal)
		}
	case "f":
		if m.activeTab == tabTimeline {
			return m.switchTimelineMode(modeFederated)
		}
	case "g":
		if m.activeTab == tabTimeline {
			return m.switchTimelineMode(modeTrending)
		}
	case "T":
		if m.activeTab == tabTimeline {
			return m.switchTimelineMode(modeTrending)
		}
	case "r":
		return m.refreshCurrent()
	}

	return m.updateActiveView(msg)
}

func (m model) updateActiveView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.activeTab {
	case tabTimeline:
		view := m.timelineView()
		view.list, cmd = view.list.Update(msg)
		if view.list.Index() != view.selected {
			view.selected = view.list.Index()
			m.renderCurrentDetail()
		}
		view.detail, _ = view.detail.Update(msg)
	case tabProfile:
		view := m.profileView
		view.list, cmd = view.list.Update(msg)
		if view.list.Index() != view.selected {
			view.selected = view.list.Index()
			m.renderCurrentDetail()
		}
		view.detail, _ = view.detail.Update(msg)
	case tabSearch:
		m.searchView.viewport, cmd = m.searchView.viewport.Update(msg)
	case tabNotifications:
		view := m.notificationsView
		view.list, cmd = view.list.Update(msg)
		if view.list.Index() != view.selected {
			view.selected = view.list.Index()
			m.renderCurrentDetail()
		}
		view.detail, _ = view.detail.Update(msg)
	}
	return m, cmd
}

func (m model) renderHeader() string {
	tabs := []string{"Timeline", "Search", "Profile", "Notifications"}
	var parts []string
	for i, name := range tabs {
		style := components.TabStyle
		if m.activeTab == topTab(i) {
			style = components.TabActiveStyle
		}
		parts = append(parts, components.RenderTabLabel(name, style))
	}
	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	tabRow = components.HeaderStyle.Render(tabRow)

	if m.activeTab != tabTimeline {
		return tabRow
	}

	modeRow := m.renderTimelineModes()
	modeRow = components.HeaderStyle.Render(modeRow)
	return tabRow + "\n" + modeRow
}

func (m model) renderContent() string {
	switch m.activeTab {
	case tabTimeline:
		return m.renderFeed(m.timelineView())
	case tabProfile:
		return m.renderFeed(m.profileView)
	case tabSearch:
		return m.searchView.viewport.View()
	case tabNotifications:
		return m.renderNotifications(m.notificationsView)
	default:
		return ""
	}
}

func (m model) renderFeed(view *feedView) string {
	leftWidth := components.Max(30, m.width/2)
	rightWidth := m.width - leftWidth - 1
	if rightWidth < 20 {
		return view.list.View()
	}

	left := lipgloss.NewStyle().Width(leftWidth).Height(m.contentHeight()).Render(view.list.View())
	right := lipgloss.NewStyle().Width(rightWidth).Height(m.contentHeight()).Render(view.detail.View())
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("│")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right)
}

func (m model) renderNotifications(view *notificationsView) string {
	leftWidth := components.Max(30, m.width/2)
	rightWidth := m.width - leftWidth - 1
	if rightWidth < 20 {
		return view.list.View()
	}

	left := lipgloss.NewStyle().Width(leftWidth).Height(m.contentHeight()).Render(view.list.View())
	right := lipgloss.NewStyle().Width(rightWidth).Height(m.contentHeight()).Render(view.detail.View())
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("│")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right)
}

func (m *model) renderCurrentDetail() {
	switch m.activeTab {
	case tabTimeline:
		m.renderDetail(m.timelineView())
	case tabProfile:
		m.renderDetail(m.profileView)
	case tabNotifications:
		m.renderNotificationsDetail(m.notificationsView)
	}
}

func (m *model) renderDetail(view *feedView) {
	if view.detail.Width == 0 {
		return
	}
	if len(view.statuses) == 0 {
		if view.loading {
			view.detail.SetContent(fmt.Sprintf("%s Loading timeline...", m.spinner.View()))
		} else {
			view.detail.SetContent("No status selected.")
		}
		return
	}

	index := view.list.Index()
	if index < 0 || index >= len(view.statuses) {
		index = 0
	}

	view.detail.SetContent(renderStatusDetail(view.statuses[index], view.detail.Width))
}

func (m *model) resizeAll() {
	for _, view := range m.timelineViews {
		m.resizeFeed(view)
	}
	m.resizeFeed(m.profileView)
	m.resizeNotifications(m.notificationsView)

	height := m.contentHeight()
	m.searchView.viewport.Width = m.width
	m.searchView.viewport.Height = components.Max(5, height)
}

func (m *model) resizeFeed(view *feedView) {
	if m.width == 0 || m.height == 0 {
		return
	}

	leftWidth := components.Max(30, m.width/2)
	rightWidth := m.width - leftWidth - 1
	view.list.SetSize(leftWidth, components.Max(5, m.contentHeight()))
	view.detail.Width = rightWidth
	view.detail.Height = components.Max(5, m.contentHeight())
}

func (m *model) resizeNotifications(view *notificationsView) {
	if m.width == 0 || m.height == 0 {
		return
	}

	leftWidth := components.Max(30, m.width/2)
	rightWidth := m.width - leftWidth - 1
	view.list.SetSize(leftWidth, components.Max(5, m.contentHeight()))
	view.detail.Width = rightWidth
	view.detail.Height = components.Max(5, m.contentHeight())
}

func (m *model) contentHeight() int {
	headerLines := 1
	if m.activeTab == tabTimeline {
		headerLines = 2
	}
	return components.Max(5, m.height-headerLines)
}

func (m model) timelineView() *feedView {
	return m.timelineViews[m.activeTimeline]
}

func (m model) isLoading() bool {
	switch m.activeTab {
	case tabTimeline:
		return m.timelineView().loading
	case tabProfile:
		return m.profileView.loading
	case tabNotifications:
		return m.notificationsView.loading
	default:
		return false
	}
}

func (m *model) ensureTabLoaded() tea.Cmd {
	switch m.activeTab {
	case tabTimeline:
		return m.ensureTimelineLoaded()
	case tabProfile:
		return m.ensureProfileLoaded()
	case tabSearch:
		return nil
	case tabNotifications:
		return m.ensureNotificationsLoaded()
	default:
		return nil
	}
}

func (m *model) refreshCurrent() (tea.Model, tea.Cmd) {
	switch m.activeTab {
	case tabTimeline:
		view := m.timelineView()
		if view.loading {
			return m, nil
		}
		view.loading = true
		view.list.StartSpinner()
		return m, tea.Batch(
			fetchTimelineCmd(m.client, m.activeTimeline, view.topID),
			m.spinner.Tick,
		)
	case tabProfile:
		view := m.profileView
		if view.loading {
			return m, nil
		}
		view.loading = true
		view.list.StartSpinner()
		return m, tea.Batch(
			fetchProfileCmd(m.client, m),
			m.spinner.Tick,
		)
	case tabNotifications:
		view := m.notificationsView
		if view.loading {
			return m, nil
		}
		view.loading = true
		view.list.StartSpinner()
		return m, tea.Batch(
			fetchNotificationsCmd(m.client),
			m.spinner.Tick,
		)
	default:
		return m, nil
	}
}
