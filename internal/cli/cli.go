package cli

import (
	"flag"
	"fmt"
	"os"

	"mastodoncli/internal/config"
	"mastodoncli/internal/mastodon"
	"mastodoncli/internal/output"
	"mastodoncli/internal/ui"
)

func Run(args []string) error {
	if len(args) < 2 {
		printUsage()
		return fmt.Errorf("missing command")
	}

	switch args[1] {
	case "login":
		return runLogin(args[2:])
	case "timeline":
		return runTimeline(args[2:])
	case "posts":
		return runPosts(args[2:])
	case "notifications":
		return runNotifications(args[2:])
	case "ui":
		return runUI(args[2:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", args[1])
	}
}

func runLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	instance := fs.String("instance", "", "Mastodon instance domain (e.g. mastodon.social)")
	force := fs.Bool("force", false, "Re-register the OAuth app even if one exists in config")
	fs.Parse(args)

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if *instance == "" {
		*instance = cfg.Instance
	}
	if *instance == "" {
		return fmt.Errorf("instance is required (use --instance)")
	}

	cfg.Instance = *instance
	if cfg.RedirectURI == "" {
		cfg.RedirectURI = "urn:ietf:wg:oauth:2.0:oob"
	}

	client := mastodon.NewClient(cfg.Instance, "")
	if cfg.ClientID == "" || cfg.ClientSecret == "" || *force {
		app, err := client.RegisterApp("MastodonCLI", cfg.RedirectURI, "read")
		if err != nil {
			return err
		}
		cfg.ClientID = app.ClientID
		cfg.ClientSecret = app.ClientSecret
	}

	authURL := client.AuthorizeURL(cfg.ClientID, cfg.RedirectURI, "read")
	fmt.Println("Open this URL in your browser and authorize the app:")
	fmt.Println(authURL)
	fmt.Println()

	code, err := prompt("Paste the authorization code: ")
	if err != nil {
		return err
	}
	if code == "" {
		return fmt.Errorf("authorization code is required")
	}

	token, err := client.ExchangeToken(cfg.ClientID, cfg.ClientSecret, cfg.RedirectURI, code, "read")
	if err != nil {
		return err
	}
	cfg.AccessToken = token.AccessToken

	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Println("Login successful. Access token saved.")
	return nil
}

func runTimeline(args []string) error {
	fs := flag.NewFlagSet("timeline", flag.ExitOnError)
	limit := fs.Int("limit", 20, "Number of statuses to fetch (1-40)")
	fs.Parse(args)

	if *limit <= 0 || *limit > 40 {
		return fmt.Errorf("limit must be between 1 and 40")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Instance == "" || cfg.AccessToken == "" {
		return fmt.Errorf("missing config; run `mastodon login --instance <domain>` first")
	}

	client := mastodon.NewClient(cfg.Instance, cfg.AccessToken)
	statuses, err := client.HomeTimeline(*limit)
	if err != nil {
		return err
	}

	output.PrintStatuses(statuses)
	return nil
}

func runPosts(args []string) error {
	fs := flag.NewFlagSet("posts", flag.ExitOnError)
	limit := fs.Int("limit", 20, "Number of statuses to fetch (1-800)")
	includeBoosts := fs.Bool("boosts", false, "Include boosts in results")
	includeReplies := fs.Bool("replies", false, "Include replies in results")
	fs.Parse(args)

	if *limit <= 0 || *limit > 800 {
		return fmt.Errorf("limit must be between 1 and 800")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Instance == "" || cfg.AccessToken == "" {
		return fmt.Errorf("missing config; run `mastodon login --instance <domain>` first")
	}

	client := mastodon.NewClient(cfg.Instance, cfg.AccessToken)
	account, err := client.VerifyCredentials()
	if err != nil {
		return err
	}

	target := *limit
	showProgress := target > 40
	pageMax := 40
	total := 0
	var maxID string
	var all []mastodon.Status

	for total < target {
		pageLimit := pageMax
		remaining := target - total
		if remaining < pageLimit {
			pageLimit = remaining
		}

		page, err := client.AccountStatuses(account.ID, pageLimit, *includeBoosts, *includeReplies, maxID)
		if err != nil {
			return err
		}
		if len(page) == 0 {
			break
		}

		all = append(all, page...)
		total += len(page)
		maxID = page[len(page)-1].ID

		if showProgress {
			fmt.Fprintf(os.Stderr, "Fetched %d/%d...\r", total, target)
		}
	}

	if showProgress {
		fmt.Fprintln(os.Stderr)
	}

	output.PrintStatuses(all)
	return nil
}

func runNotifications(args []string) error {
	fs := flag.NewFlagSet("notifications", flag.ExitOnError)
	limit := fs.Int("limit", 20, "Number of notifications to fetch (1-40)")
	fs.Parse(args)

	if *limit <= 0 || *limit > 40 {
		return fmt.Errorf("limit must be between 1 and 40")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Instance == "" || cfg.AccessToken == "" {
		return fmt.Errorf("missing config; run `mastodon login --instance <domain>` first")
	}

	client := mastodon.NewClient(cfg.Instance, cfg.AccessToken)
	notifications, err := client.GroupedNotifications(*limit)
	if err != nil {
		return err
	}

	output.PrintNotifications(notifications)
	return nil
}

func runUI(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("ui does not accept arguments")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Instance == "" || cfg.AccessToken == "" {
		return fmt.Errorf("missing config; run `mastodon login --instance <domain>` first")
	}

	client := mastodon.NewClient(cfg.Instance, cfg.AccessToken)
	return ui.Run(client)
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  mastodon login --instance <domain> [--force]")
	fmt.Println("  mastodon timeline --limit <n>")
	fmt.Println("  mastodon posts --limit <n> [--boosts] [--replies]")
	fmt.Println("  mastodon notifications --limit <n>")
	fmt.Println("  mastodon ui")
}
