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
	"mastodoncli/internal/ui/components"
)

type topTab int

type timelineMode int

const (
	tabTimeline topTab = iota
	tabSearch
	tabProfile
)

const (
	modeHome timelineMode = iota
	modeLocal
	modeFederated
	modeTrending
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

type searchView struct {
	viewport viewport.Model
}

type model struct {
	client           *mastodon.Client
	activeTab        topTab
	activeTimeline   timelineMode
	timelineViews    map[timelineMode]*feedView
	profileView      *feedView
	searchView       searchView
	profileAccountID string
	spinner          spinner.Model
	width            int
	height           int
}

type timelineMsg struct {
	mode     timelineMode
	statuses []mastodon.Status
	sinceID  string
}

type profileMsg struct {
	statuses  []mastodon.Status
	accountID string
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
	search := searchView{viewport: viewport.New(0, 0)}

	return model{
		client:         client,
		activeTab:      tabTimeline,
		activeTimeline: modeHome,
		timelineViews:  timelineViews,
		profileView:    profile,
		searchView:     search,
		spinner:        sp,
	}
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
		m.activeTab = (m.activeTab + 1) % 3
		m.resizeAll()
		m.renderCurrentDetail()
		m.renderSearch()
		return m, m.ensureTabLoaded()
	case "shift+tab":
		m.activeTab = (m.activeTab + 2) % 3
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
	}
	return m, cmd
}

func (m model) renderHeader() string {
	tabs := []string{"Timeline", "Search", "Profile"}
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

func (m model) renderContent() string {
	switch m.activeTab {
	case tabTimeline:
		return m.renderFeed(m.timelineView())
	case tabProfile:
		return m.renderFeed(m.profileView)
	case tabSearch:
		return m.searchView.viewport.View()
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

func (m *model) renderCurrentDetail() {
	switch m.activeTab {
	case tabTimeline:
		m.renderDetail(m.timelineView())
	case tabProfile:
		m.renderDetail(m.profileView)
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

func (m *model) renderSearch() {
	if m.searchView.viewport.Width == 0 {
		return
	}
	content := "Search\n\n" +
		"This tab will let you search accounts, hashtags, and statuses.\n" +
		"(Coming soon)"
	m.searchView.viewport.SetContent(content)
}

func (m *model) resizeAll() {
	for _, view := range m.timelineViews {
		m.resizeFeed(view)
	}
	m.resizeFeed(m.profileView)

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

func (m model) isLoading() bool {
	switch m.activeTab {
	case tabTimeline:
		return m.timelineView().loading
	case tabProfile:
		return m.profileView.loading
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
	default:
		return nil
	}
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

func (m *model) ensureProfileLoaded() tea.Cmd {
	view := m.profileView
	if !view.loading && len(view.statuses) > 0 {
		return nil
	}
	view.loading = true
	view.list.SetItems([]list.Item{loadingItem()})
	view.list.StartSpinner()
	return tea.Batch(
		fetchProfileCmd(m.client, m),
		m.spinner.Tick,
	)
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
	default:
		return m, nil
	}
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
			if sinceID != "" {
				statuses, err = client.TrendingStatuses(40)
			} else {
				statuses, err = client.TrendingStatuses(40)
			}
		default:
			statuses = nil
		}
		if err != nil {
			return feedErrMsg{tab: tabTimeline, mode: mode, err: err}
		}
		return timelineMsg{mode: mode, statuses: statuses, sinceID: sinceID}
	}
}

func fetchProfileCmd(client *mastodon.Client, m *model) tea.Cmd {
	return func() tea.Msg {
		accountID := m.profileAccountID
		if m.profileAccountID == "" {
			acct, err := client.VerifyCredentials()
			if err != nil {
				return feedErrMsg{tab: tabProfile, err: err}
			}
			accountID = acct.ID
		}

		statuses, err := client.AccountStatuses(accountID, 40, false, false, "")
		if err != nil {
			return feedErrMsg{tab: tabProfile, err: err}
		}
		return profileMsg{statuses: statuses, accountID: accountID}
	}
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
