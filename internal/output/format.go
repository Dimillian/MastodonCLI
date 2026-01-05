package output

import (
	"fmt"
	"html"
	"strings"

	"mastodoncli/internal/mastodon"
)

func PrintStatuses(statuses []mastodon.Status) {
	if len(statuses) == 0 {
		fmt.Println("No statuses returned.")
		return
	}

	for _, item := range statuses {
		display := &item
		boostedBy := ""
		if item.Reblog != nil {
			boostedBy = fmt.Sprintf("@%s", item.Account.Acct)
			display = item.Reblog
		}

		name := strings.TrimSpace(StripHTML(display.Account.DisplayName))
		body := WrapText(StripHTML(display.Content), 80)

		fmt.Println("----")
		if name != "" && name != display.Account.Acct {
			fmt.Printf("%sAuthor:%s %s (@%s)\n", colorCyan, colorReset, name, display.Account.Acct)
		} else {
			fmt.Printf("%sAuthor:%s @%s\n", colorCyan, colorReset, display.Account.Acct)
		}
		fmt.Printf("%sTime:%s   %s\n", colorYellow, colorReset, display.CreatedAt)
		if boostedBy != "" {
			fmt.Printf("Boost:  %s\n", boostedBy)
		}
		fmt.Println("Text:")
		fmt.Println(body)
		fmt.Println()
	}
}

func PrintNotifications(notifications []mastodon.GroupedNotification) {
	if len(notifications) == 0 {
		fmt.Println("No notifications returned.")
		return
	}

	for _, item := range notifications {
		fmt.Println("----")
		fmt.Printf("%sType:%s  %s (%d)\n", colorCyan, colorReset, notificationTypeLabel(item.Type), item.Count)
		fmt.Printf("%sFrom:%s  %s\n", colorCyan, colorReset, notificationAccountsLabel(item.Accounts))
		if item.LatestAt != "" {
			fmt.Printf("%sTime:%s  %s\n", colorYellow, colorReset, item.LatestAt)
		} else {
			fmt.Printf("%sTime:%s  Unknown\n", colorYellow, colorReset)
		}

		if item.Status != nil {
			fmt.Println("Text:")
			body := WrapText(StripHTML(item.Status.Content), 80)
			if body == "" {
				body = "(no text)"
			}
			fmt.Println(body)
		}
		fmt.Println()
	}
}

func StripHTML(input string) string {
	var builder strings.Builder
	builder.Grow(len(input))

	inTag := false
	for _, r := range input {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				builder.WriteRune(r)
			}
		}
	}

	return strings.TrimSpace(html.UnescapeString(builder.String()))
}

func WrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var builder strings.Builder
	lineLen := 0
	for _, word := range words {
		if lineLen == 0 {
			builder.WriteString(word)
			lineLen = len(word)
			continue
		}

		if lineLen+1+len(word) > width {
			builder.WriteByte('\n')
			builder.WriteString(word)
			lineLen = len(word)
			continue
		}

		builder.WriteByte(' ')
		builder.WriteString(word)
		lineLen += 1 + len(word)
	}

	return builder.String()
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

func formatAccount(account mastodon.Account) string {
	name := strings.TrimSpace(StripHTML(account.DisplayName))
	if name != "" && name != account.Acct {
		return fmt.Sprintf("%s (@%s)", name, account.Acct)
	}
	return fmt.Sprintf("@%s", account.Acct)
}

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorYellow = "\033[33m"
)
