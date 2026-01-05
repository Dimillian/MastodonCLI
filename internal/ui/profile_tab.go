package ui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"mastodoncli/internal/mastodon"
)

type profileMsg struct {
	statuses  []mastodon.Status
	accountID string
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
