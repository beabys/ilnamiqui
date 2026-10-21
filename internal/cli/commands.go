package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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
	KEYS    = "keys"
	KEY     = "key"
	SEARCH  = "search"
	PRUNE   = "prune"
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
	case KEYS:
		return c.cmdKeys(cmdArgs)
	case KEY:
		return c.cmdKey(cmdArgs)
	case SEARCH:
		return c.cmdSearch(cmdArgs)
	case PRUNE:
		return c.cmdPrune(cmdArgs)
	case DELETE:
		return c.cmdDelete(cmdArgs)
	case SESSION:
		return c.cmdSession(cmdArgs)
	case VERSION:
		return cmdVersion()
	case HELP, "--help", "-h":
		if len(cmdArgs) > 0 {
			return printCommandHelp(cmdArgs[0])
		}
		return printHelp()
	default:
		return fmt.Errorf("unknown command %q — run 'ilnamiqui help' for usage", cmd)
	}
}

// cmdInit initializes the database.
func (c *CLI) cmdInit(args []string) error {
	fs := newCommandFS("init")
	if err := fs.Parse(args); err != nil {
		return helpOrErr("init", err)
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
	fs := newCommandFS("save")
	pretty := fs.Bool("pretty", false, "human-readable output")
	agent := fs.String("agent", "opencode", "agent identifier")
	positionals, err := parseFlagsAnywhere(fs, args, map[string]bool{"agent": true})
	if err != nil {
		return helpOrErr("save", err)
	}
	if len(positionals) < 2 {
		return fmt.Errorf("usage: ilnamiqui save <key> <value>")
	}
	key := positionals[0]
	value := strings.Join(positionals[1:], " ")

	resp, err := c.svc.Save(context.Background(), &service.SaveRequest{Key: key, Value: value, Agent: *agent})
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
	fs := newCommandFS("load")
	sessionFlag := fs.Bool("session", false, "load entries for active session only")
	pretty := fs.Bool("pretty", false, "human-readable output")
	limit := fs.Int("limit", 0, "maximum number of entries to return (0 = no limit)")
	if err := fs.Parse(args); err != nil {
		return helpOrErr("load", err)
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
	fs := newCommandFS("list")
	limit := fs.Int("limit", 10, "number of sessions to list")
	pretty := fs.Bool("pretty", false, "human-readable output")
	if err := fs.Parse(args); err != nil {
		return helpOrErr("list", err)
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

// cmdKeys lists distinct memory keys.
func (c *CLI) cmdKeys(args []string) error {
	fs := newCommandFS("keys")
	pretty := fs.Bool("pretty", false, "human-readable output")
	limit := fs.Int("limit", 0, "maximum number of keys to return (0 = no limit)")
	if err := fs.Parse(args); err != nil {
		return helpOrErr("keys", err)
	}

	resp, err := c.svc.ListKeys(context.Background(), &service.ListKeysRequest{Limit: *limit})
	if err != nil {
		return err
	}

	if *pretty {
		return printKeysTable(resp.Keys)
	}
	return printJSON(resp.Keys)
}

// cmdKey handles key subcommands.
func (c *CLI) cmdKey(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ilnamiqui key <subcommand> [flags]")
	}
	sub := args[0]
	subArgs := args[1:]
	switch sub {
	case "update":
		return c.cmdKeyUpdate(subArgs)
	default:
		return fmt.Errorf("unknown key subcommand %q — use 'update'", sub)
	}
}

func (c *CLI) cmdKeyUpdate(args []string) error {
	fs := newCommandFS("key update")
	critical := fs.Bool("critical", false, "set critical flag (use --critical or --critical=true)")
	if err := fs.Parse(args); err != nil {
		return helpOrErr("key update", err)
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: ilnamiqui key update [--critical[=true|false]] <keyname>")
	}
	key := fs.Arg(0)
	_, err := c.svc.KeyUpdate(context.Background(), &service.KeyUpdateRequest{
		Key:      key,
		Critical: *critical,
	})
	if err != nil {
		return err
	}
	fmt.Printf("updated key %q: critical=%v\n", key, *critical)
	return nil
}

// cmdSearch searches memory entries.
func (c *CLI) cmdSearch(args []string) error {
	fs := newCommandFS("search")
	pretty := fs.Bool("pretty", false, "human-readable output")
	limit := fs.Int("limit", 0, "maximum number of entries to return (0 = no limit)")
	after := fs.String("after", "", "only entries after this date (RFC3339 or YYYY-MM-DD)")
	before := fs.String("before", "", "only entries before this date (RFC3339 or YYYY-MM-DD)")
	mode := fs.String("mode", "key", "search mode: key (default, uses index), content (value FTS5), both (key+content)")
	positionals, err := parseFlagsAnywhere(fs, args, map[string]bool{
		"limit": true, "after": true, "before": true, "mode": true,
	})
	if err != nil {
		return helpOrErr("search", err)
	}

	query := strings.Join(positionals, " ")

	switch *mode {
	case "key", "content", "both":
		// valid
	default:
		return fmt.Errorf("invalid --mode %q: must be key, content, or both", *mode)
	}

	// Require at least a query, --after, or --before
	if query == "" && *after == "" && *before == "" {
		return fmt.Errorf("usage: ilnamiqui search [--pretty] [--mode key|content|both] [--limit N] [--after DATE] [--before DATE] <query>")
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
		Mode:   memory.SearchMode(*mode),
		Limit:  *limit,
		After:  afterTime,
		Before: beforeTime,
	})
	if err != nil {
		return err
	}

	if len(resp.Entries) == 0 && *mode == "key" && query != "" {
		fmt.Fprintln(os.Stderr, "no entries found — try --mode both to search values")
	}

	if *pretty {
		return printEntriesTable(resp.Entries)
	}
	return printJSON(resp.Entries)
}

// cmdPrune deletes non-critical memory entries older than the given date.
func (c *CLI) cmdPrune(args []string) error {
	fs := newCommandFS("prune")
	before := fs.String("before", "", "required — delete entries older than this date (YYYY-MM-DD or RFC3339)")
	key := fs.String("key", "*", "key to prune (default: all non-critical keys)")
	if err := fs.Parse(args); err != nil {
		return helpOrErr("prune", err)
	}
	if *before == "" {
		return fmt.Errorf("usage: ilnamiqui prune --before <date> [--key <key>]")
	}
	beforeTime, err := parseDate(*before)
	if err != nil {
		return fmt.Errorf("invalid --before: %w", err)
	}
	resp, err := c.svc.Prune(context.Background(), &service.PruneRequest{
		Before: beforeTime,
		Key:    *key,
	})
	if err != nil {
		return err
	}
	fmt.Printf("pruned %d entries, cleaned %d orphan keys\n", resp.Deleted, resp.OrphansCleaned)
	return nil
}

// cmdDelete deletes a memory entry.
func (c *CLI) cmdDelete(args []string) error {
	fs := newCommandFS("delete")
	if err := fs.Parse(args); err != nil {
		return helpOrErr("delete", err)
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
	fs := newCommandFS("session start")
	agent := fs.String("agent", "opencode", "agent identifier")
	if err := fs.Parse(args); err != nil {
		return helpOrErr("session start", err)
	}
	resp, err := c.svc.StartSession(context.Background(), &service.StartSessionRequest{Agent: *agent})
	if err != nil {
		return err
	}
	fmt.Println(resp.Session.ID)
	return nil
}

func (c *CLI) cmdSessionEnd(args []string) error {
	fs := newCommandFS("session end")
	summary := fs.String("summary", "", "session summary")
	agent := fs.String("agent", "opencode", "agent identifier")
	if err := fs.Parse(args); err != nil {
		return helpOrErr("session end", err)
	}
	resp, err := c.svc.EndSession(context.Background(), &service.EndSessionRequest{Summary: *summary, Agent: *agent})
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
  init                                                   Initialize database in .ilnamiqui/
  save [--agent <agent>] <key> <value>                   Save a memory entry
  load [--session] [--limit N] [--pretty]                Load memory entries (all or current session)
  list [--limit N] [--pretty]                            List recent sessions
  keys [--limit N] [--pretty]                            List distinct memory keys
  key update [--critical[=true|false]] <key>             Update critical flag on a memory key (default: false)
  search [--mode key|content|both] [--pretty] [--limit N] [--after DATE] [--before DATE] <query>
                                                         Search memory entries by key or value
  prune --before <date> [--key <key>]                    Delete non-critical memory entries older than date
  delete <id>                                            Delete a memory entry by ID
  session start [--agent <agent>]                        Start a new session
  session end [--agent <agent>] [--summary "text"]       End the active session
  version                                                Print version
  help [command]                                         Print this help, or help for a specific command

Global flags:
  --pretty              Human-readable output (tables) instead of JSON (supported by most commands)
  --                    Terminate flag scanning; everything after is treated as positional arguments

Notes:
  - Flags may appear before or after positional arguments for commands that support them (see "help <command>").
  - Use "--" to pass arguments that begin with "-" as positionals (e.g. "ilnamiqui search -- -foo").
  - Run "ilnamiqui help <command>" for detailed usage of a specific command.
`
	fmt.Print(help)
	return nil
}

// newCommandFS creates a FlagSet with ContinueOnError and output discarded.
// Used by all subcommands so that --help triggers ErrHelp instead of exiting.
func newCommandFS(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// helpOrErr converts flag.ErrHelp into a call to printCommandHelp(name).
// Other errors pass through unchanged.
func helpOrErr(name string, err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return printCommandHelp(name)
	}
	return err
}

// printCommandHelp prints help for a specific command.
// Returns error for unknown commands.
func printCommandHelp(name string) error {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return fmt.Errorf("unknown command %q — run 'ilnamiqui help' for usage", name)
	}
	cmd := parts[0]
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}

	var help string
	switch cmd {
	case "init":
		help = `ilnamiqui init

Usage:
  ilnamiqui init

Initialize the ilnamiqui database in .ilnamiqui/ directory.

Flags:
  (none)
`
	case "save":
		help = `ilnamiqui save

Usage:
  ilnamiqui save [--agent <agent>] <key> <value>

Save a memory entry. The --agent flag defaults to "opencode".

Flags:
  --agent <agent>   Agent identifier (default: "opencode")
  --pretty          Human-readable output instead of JSON

Notes:
  - Flags may appear before or after positional arguments.
  - Use "--" to pass values starting with "-" as the key or value.
`
	case "load":
		help = `ilnamiqui load

Usage:
  ilnamiqui load [--session] [--limit N] [--pretty]

Load memory entries. Use --session to load only entries from the current session.

Flags:
  --session         Load entries for active session only
  --limit N         Maximum number of entries to return (0 = no limit, default: 0)
  --pretty          Human-readable output instead of JSON
`
	case "list":
		help = `ilnamiqui list

Usage:
  ilnamiqui list [--limit N] [--pretty]

List recent sessions.

Flags:
  --limit N         Number of sessions to list (default: 10)
  --pretty          Human-readable output instead of JSON
`
	case "keys":
		help = `ilnamiqui keys

Usage:
  ilnamiqui keys [--limit N] [--pretty]

List distinct memory keys.

Flags:
  --limit N         Maximum number of keys to return (0 = no limit, default: 0)
  --pretty          Human-readable output instead of JSON
`
	case "key":
		if sub == "update" || sub == "" {
			help = `ilnamiqui key update

Usage:
  ilnamiqui key update [--critical[=true|false]] <key>

Update the critical flag on a memory key. Critical keys are never pruned.

Flags:
  --critical[=true|false]   Set critical flag (default: false)
`
		} else {
			return fmt.Errorf("unknown key subcommand %q — use 'update'", sub)
		}
	case "search":
		help = `ilnamiqui search

Usage:
  ilnamiqui search [--mode key|content|both] [--pretty] [--limit N] [--after DATE] [--before DATE] <query>

Search memory entries by key or value.

Modes:
  key       Prefix match on key (indexed, fast). Default mode.
  content   FTS5 token-prefix search over key + value (slower, full-text).
  both      Union of key and content modes.

Flags:
  --mode key|content|both   Search mode (default: key)
  --pretty                  Human-readable output instead of JSON
  --limit N                 Maximum number of entries to return (0 = no limit, default: 0)
  --after DATE              Only entries after this date (RFC3339 or YYYY-MM-DD)
  --before DATE             Only entries before this date (RFC3339 or YYYY-MM-DD)

Notes:
  - Flags may appear before or after the query.
  - Use "--" to pass queries starting with "-" (e.g. "ilnamiqui search -- -foo").
  - When mode=key returns no results, a hint suggests trying --mode both.
`
	case "prune":
		help = `ilnamiqui prune

Usage:
  ilnamiqui prune --before <date> [--key <key>]

Delete non-critical memory entries older than the specified date.

Flags:
  --before <date>   Required — delete entries older than this date (YYYY-MM-DD or RFC3339)
  --key <key>       Key to prune (default: all non-critical keys)
`
	case "delete":
		help = `ilnamiqui delete

Usage:
  ilnamiqui delete <id>

Delete a memory entry by ID.

Arguments:
  <id>   Entry ID (integer)
`
	case "session":
		switch sub {
		case "start", "":
			help = `ilnamiqui session start

Usage:
  ilnamiqui session start [--agent <agent>]

Start a new session.

Flags:
  --agent <agent>   Agent identifier (default: "opencode")
`
		case "end":
			help = `ilnamiqui session end

Usage:
  ilnamiqui session end [--agent <agent>] [--summary "text"]

End the active session.

Flags:
  --agent <agent>       Agent identifier (default: "opencode")
  --summary "text"      Session summary
`
		default:
			return fmt.Errorf("unknown session subcommand %q — use 'start' or 'end'", sub)
		}
	case "version":
		help = `ilnamiqui version

Usage:
  ilnamiqui version

Print the ilnamiqui version.
`
	case "help":
		help = `ilnamiqui help

Usage:
  ilnamiqui help [command]

Print help for ilnamiqui or a specific command.

Examples:
  ilnamiqui help          Print top-level help
  ilnamiqui help search   Print help for the search command
`
	default:
		return fmt.Errorf("unknown command %q — run 'ilnamiqui help' for usage", name)
	}
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

// parseFlagsAnywhere parses flags from args regardless of position.
// Tokens starting with "-" (except bare "-") are treated as flags.
// "--" stops flag scanning; everything after is positional.
// valueFlags lists flag names that consume the next token as their value.
// Unknown flags are passed through to fs.Parse, which will report an error.
func parseFlagsAnywhere(fs *flag.FlagSet, args []string, valueFlags map[string]bool) ([]string, error) {
	var flagTokens []string
	var positionals []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// "--" terminator: rest are positionals
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}

		// Flag detection: starts with "-" but not bare "-"
		if len(arg) > 1 && arg[0] == '-' {
			flagTokens = append(flagTokens, arg)

			// If contains "=", value is embedded — don't consume next
			if strings.Contains(arg, "=") {
				continue
			}

			// Extract flag name
			name := arg
			if strings.HasPrefix(name, "--") {
				name = name[2:]
			} else {
				name = name[1:]
			}

			// If flag takes a value, consume next token
			if valueFlags[name] && i+1 < len(args) {
				i++
				flagTokens = append(flagTokens, args[i])
			}
			continue
		}

		// Positional
		positionals = append(positionals, arg)
	}

	if err := fs.Parse(flagTokens); err != nil {
		return nil, err
	}
	return positionals, nil
}

// printKeysTable writes keys as a human-readable table.
func printKeysTable(keys []memory.KeyInfo) error {
	if len(keys) == 0 {
		fmt.Println("no keys found")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "Key\tCritical\tLast Used"); err != nil {
		return err
	}
	for _, k := range keys {
		critical := "false"
		if k.Critical {
			critical = "true"
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", k.Key, critical, k.LastUsedAt.Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return w.Flush()
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
