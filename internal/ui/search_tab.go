package ui

import "github.com/charmbracelet/bubbles/viewport"

type searchView struct {
	viewport viewport.Model
}

func newSearchView() searchView {
	return searchView{viewport: viewport.New(0, 0)}
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
