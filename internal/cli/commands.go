package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/beabys/ilnamiqui/internal/memory"
	"github.com/beabys/ilnamiqui/internal/service"
)

const (
	INIT    = "init"
	SAVE    = "save"
	LOAD    = "load"
	LIST    = "list"
	SEARCH  = "search"
	DELETE  = "delete"
	SESSION = "session"
	VERSION = "version"
	HELP    = "help"
)

var version = "dev"

// CLI handles command-line parsing and output formatting.
type CLI struct {
	svc service.Service
}

// New creates a CLI with the given service.
func New(svc service.Service) *CLI {
	return &CLI{svc: svc}
}

// Run is the main CLI entry point. It parses args and dispatches to subcommands.
func (c *CLI) Run(args []string) error {
	if len(args) == 0 {
		return printHelp()
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case INIT:
		return c.cmdInit(cmdArgs)
	case SAVE:
		return c.cmdSave(cmdArgs)
	case LOAD:
		return c.cmdLoad(cmdArgs)
	case LIST:
		return c.cmdList(cmdArgs)
	case SEARCH:
		return c.cmdSearch(cmdArgs)
	case DELETE:
		return c.cmdDelete(cmdArgs)
	case SESSION:
		return c.cmdSession(cmdArgs)
	case VERSION:
		return cmdVersion()
	case HELP, "--help", "-h":
		return printHelp()
	default:
		return fmt.Errorf("unknown command %q — run 'ilnamiqui help' for usage", cmd)
	}
}

// cmdInit initializes the database.
func (c *CLI) cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	resp, err := c.svc.Init(context.Background(), &service.InitRequest{})
	if err != nil {
		return err
	}
	fmt.Printf("ilnamiqui initialized at %s\n", resp.DBPath)
	return nil
}

// cmdSave saves a memory entry.
func (c *CLI) cmdSave(args []string) error {
	fs := flag.NewFlagSet("save", flag.ContinueOnError)
	pretty := fs.Bool("pretty", false, "human-readable output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return fmt.Errorf("usage: ilnamiqui save <key> <value>")
	}
	key := fs.Arg(0)
	value := strings.Join(fs.Args()[1:], " ")

	resp, err := c.svc.Save(context.Background(), &service.SaveRequest{Key: key, Value: value})
	if err != nil {
		return err
	}

	if *pretty {
		fmt.Printf("ID        %d\n", resp.Entry.ID)
		fmt.Printf("Key       %s\n", resp.Entry.Key)
		fmt.Printf("Value     %s\n", resp.Entry.Value)
		fmt.Printf("Session   %s\n", resp.Entry.SessionID)
		fmt.Printf("Created   %s\n", resp.Entry.CreatedAt.Format(time.RFC3339))
	} else {
		return printJSON(resp.Entry)
	}
	return nil
}

// cmdLoad loads memory entries.
func (c *CLI) cmdLoad(args []string) error {
	fs := flag.NewFlagSet("load", flag.ContinueOnError)
	sessionFlag := fs.Bool("session", false, "load entries for active session only")
	pretty := fs.Bool("pretty", false, "human-readable output")
	limit := fs.Int("limit", 0, "maximum number of entries to return (0 = no limit)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	resp, err := c.svc.Load(context.Background(), &service.LoadRequest{
		Limit:       *limit,
		SessionOnly: *sessionFlag,
	})
	if err != nil {
		return err
	}

	if *pretty {
		return printEntriesTable(resp.Entries)
	}
	return printJSON(resp.Entries)
}

// cmdList lists recent sessions.
func (c *CLI) cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	limit := fs.Int("limit", 10, "number of sessions to list")
	pretty := fs.Bool("pretty", false, "human-readable output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	resp, err := c.svc.ListSessions(context.Background(), &service.ListSessionsRequest{Limit: *limit})
	if err != nil {
		return err
	}

	if *pretty {
		return printSessionsTable(resp.Sessions)
	}
	return printJSON(resp.Sessions)
}

// cmdSearch searches memory entries.
func (c *CLI) cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	pretty := fs.Bool("pretty", false, "human-readable output")
	limit := fs.Int("limit", 0, "maximum number of entries to return (0 = no limit)")
	after := fs.String("after", "", "only entries after this date (RFC3339 or YYYY-MM-DD)")
	before := fs.String("before", "", "only entries before this date (RFC3339 or YYYY-MM-DD)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	query := strings.Join(fs.Args(), " ")

	// Require at least a query, --after, or --before
	if query == "" && *after == "" && *before == "" {
		return fmt.Errorf("usage: ilnamiqui search <query> [--after DATE] [--before DATE] [--limit N]")
	}

	var afterTime, beforeTime *time.Time
	if *after != "" {
		t, err := parseDate(*after)
		if err != nil {
			return fmt.Errorf("invalid --after: %w", err)
		}
		afterTime = &t
	}
	if *before != "" {
		t, err := parseDate(*before)
		if err != nil {
			return fmt.Errorf("invalid --before: %w", err)
		}
		beforeTime = &t
	}

	resp, err := c.svc.Search(context.Background(), &service.SearchRequest{
		Query:  query,
		Limit:  *limit,
		After:  afterTime,
		Before: beforeTime,
	})
	if err != nil {
		return err
	}

	if *pretty {
		return printEntriesTable(resp.Entries)
	}
	return printJSON(resp.Entries)
}

// cmdDelete deletes a memory entry.
func (c *CLI) cmdDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: ilnamiqui delete <id>")
	}
	var id int64
	if _, err := fmt.Sscanf(fs.Arg(0), "%d", &id); err != nil {
		return fmt.Errorf("invalid id %q: %w", fs.Arg(0), err)
	}

	_, err := c.svc.Delete(context.Background(), &service.DeleteRequest{ID: id})
	if err != nil {
		return err
	}
	fmt.Printf("deleted entry %d\n", id)
	return nil
}

// cmdSession handles session subcommands.
func (c *CLI) cmdSession(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ilnamiqui session <start|end>")
	}
	sub := args[0]
	subArgs := args[1:]
	switch sub {
	case "start":
		return c.cmdSessionStart(subArgs)
	case "end":
		return c.cmdSessionEnd(subArgs)
	default:
		return fmt.Errorf("unknown session subcommand %q — use 'start' or 'end'", sub)
	}
}

func (c *CLI) cmdSessionStart(args []string) error {
	fs := flag.NewFlagSet("session start", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	resp, err := c.svc.StartSession(context.Background(), &service.StartSessionRequest{})
	if err != nil {
		return err
	}
	fmt.Println(resp.Session.ID)
	return nil
}

func (c *CLI) cmdSessionEnd(args []string) error {
	fs := flag.NewFlagSet("session end", flag.ContinueOnError)
	summary := fs.String("summary", "", "session summary")
	if err := fs.Parse(args); err != nil {
		return err
	}
	resp, err := c.svc.EndSession(context.Background(), &service.EndSessionRequest{Summary: *summary})
	if err != nil {
		return err
	}
	fmt.Printf("ended session %s\n", resp.Session.ID)
	return nil
}

// cmdVersion prints the CLI version.
func cmdVersion() error {
	fmt.Println(version)
	return nil
}

// printHelp prints usage information.
func printHelp() error {
	const help = `ilnamiqui — session memory for opencode (Nahuatl: "to remember")

Usage:
  ilnamiqui <command> [flags]

Commands:
  init                  Initialize database in .ilnamiqui/
  save <key> <value>    Save a memory entry
  load [--session] [--limit N]
                        Load memory entries (all or current session)
  list [--limit N]      List recent sessions
  search <query> [--after DATE] [--before DATE] [--limit N]
                        Search memory entries by key or value, optionally filtered by date
  delete <id>           Delete a memory entry by ID
  session start         Start a new session
  session end [--summary "text"]
                        End the active session
  version               Print version
  help                  Print this help

Flags:
  --pretty              Human-readable output (tables) instead of JSON

Use "ilnamiqui help <command>" for more details on a specific command.
`
	fmt.Print(help)
	return nil
}

// parseDate parses a date string in RFC3339 or YYYY-MM-DD format.
func parseDate(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse("2006-01-02", s)
	if err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 (2006-01-02T15:04:05Z) or YYYY-MM-DD (2006-01-02), got %q", s)
}

// printJSON writes v as indented JSON to stdout.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printEntriesTable writes entries as a human-readable table.
func printEntriesTable(entries []memory.MemoryEntry) error {
	if len(entries) == 0 {
		fmt.Println("no entries found")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tKey\tValue\tSession ID\tCreated")
	for _, e := range entries {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", e.ID, e.Key, e.Value, e.SessionID, e.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

// printSessionsTable writes sessions as a human-readable table.
func printSessionsTable(sessions []memory.Session) error {
	if len(sessions) == 0 {
		fmt.Println("no sessions found")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tProject\tStarted\tEnded\tSummary")
	for _, s := range sessions {
		ended := ""
		if s.EndedAt != nil {
			ended = s.EndedAt.Format(time.RFC3339)
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.ID, s.Project, s.StartedAt.Format(time.RFC3339), ended, s.Summary)
	}
	return w.Flush()
}
