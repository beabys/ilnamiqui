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

	"github.com/beabys/ilnamiqui/internal/config"
	"github.com/beabys/ilnamiqui/internal/db"
	"github.com/beabys/ilnamiqui/internal/memory"
	"github.com/beabys/ilnamiqui/internal/session"
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

// Run is the main CLI entry point. It parses args and dispatches to subcommands.
func Run(args []string) error {
	if len(args) == 0 {
		return printHelp()
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case INIT:
		return cmdInit(cmdArgs)
	case SAVE:
		return cmdSave(cmdArgs)
	case LOAD:
		return cmdLoad(cmdArgs)
	case LIST:
		return cmdList(cmdArgs)
	case SEARCH:
		return cmdSearch(cmdArgs)
	case DELETE:
		return cmdDelete(cmdArgs)
	case SESSION:
		return cmdSession(cmdArgs)
	case VERSION:
		return cmdVersion()
	case HELP, "--help", "-h":
		return printHelp()
	default:
		return fmt.Errorf("unknown command %q — run 'ilnamiqui help' for usage", cmd)
	}
}

// cmdInit initializes the database in .opencode/ilnamiqui.db, creating the directory if needed.
func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	root, err := config.FindProjectRoot()
	if err != nil {
		// Create .opencode directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getwd: %w", err)
		}
		root = cwd
		if err := os.MkdirAll(root+"/.opencode", 0o755); err != nil {
			return fmt.Errorf("create .opencode: %w", err)
		}
	}

	dbPath := root + "/.opencode/ilnamiqui.db"
	database, err := db.NewDB(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database.SQLDB()); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	fmt.Printf("ilnamiqui initialized at %s\n", dbPath)
	return nil
}

// cmdSave saves a memory entry for the active session.
func cmdSave(args []string) error {
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

	database, projectSlug, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	mgr := session.NewManager(database.SQLDB())
	sess, err := mgr.GetActiveSession(ctx, projectSlug)
	if err != nil {
		return fmt.Errorf("get active session: %w", err)
	}

	store := memory.NewStore(database.SQLDB())
	entry, err := store.SaveEntry(ctx, sess.ID, key, value)
	if err != nil {
		return fmt.Errorf("save entry: %w", err)
	}

	if *pretty {
		fmt.Printf("ID        %d\n", entry.ID)
		fmt.Printf("Key       %s\n", entry.Key)
		fmt.Printf("Value     %s\n", entry.Value)
		fmt.Printf("Session   %s\n", entry.SessionID)
		fmt.Printf("Created   %s\n", entry.CreatedAt.Format(time.RFC3339))
	} else {
		return printJSON(entry)
	}
	return nil
}

// cmdLoad loads and prints memory entries, either for the active session or all sessions.
func cmdLoad(args []string) error {
	fs := flag.NewFlagSet("load", flag.ContinueOnError)
	sessionFlag := fs.Bool("session", false, "load entries for active session only")
	pretty := fs.Bool("pretty", false, "human-readable output")
	limit := fs.Int("limit", 0, "maximum number of entries to return (0 = no limit)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	database, projectSlug, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	store := memory.NewStore(database.SQLDB())

	var entries []memory.MemoryEntry

	if *sessionFlag {
		mgr := session.NewManager(database.SQLDB())
		sess, err := mgr.GetActiveSession(ctx, projectSlug)
		if err != nil {
			return fmt.Errorf("get active session: %w", err)
		}
		entries, err = store.LoadEntries(ctx, sess.ID, *limit)
		if err != nil {
			return fmt.Errorf("load entries: %w", err)
		}
	} else {
		var err error
		entries, err = store.LoadAllEntries(ctx, *limit)
		if err != nil {
			return fmt.Errorf("load all entries: %w", err)
		}
	}

	if *pretty {
		return printEntriesTable(entries)
	}
	return printJSON(entries)
}

// cmdList lists recent sessions for the project.
func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	limit := fs.Int("limit", 10, "number of sessions to list")
	pretty := fs.Bool("pretty", false, "human-readable output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	database, projectSlug, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	mgr := session.NewManager(database.SQLDB())
	sessions, err := mgr.ListSessions(ctx, projectSlug, *limit)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if *pretty {
		return printSessionsTable(sessions)
	}
	return printJSON(sessions)
}

// cmdSearch searches memory entries by key or value.
func cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	pretty := fs.Bool("pretty", false, "human-readable output")
	limit := fs.Int("limit", 0, "maximum number of entries to return (0 = no limit)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: ilnamiqui search <query>")
	}

	query := fs.Arg(0)

	database, _, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	store := memory.NewStore(database.SQLDB())
	entries, err := store.SearchEntries(ctx, query, *limit)
	if err != nil {
		return fmt.Errorf("search entries: %w", err)
	}

	if *pretty {
		return printEntriesTable(entries)
	}
	return printJSON(entries)
}

// cmdDelete deletes a memory entry by ID.
func cmdDelete(args []string) error {
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

	database, _, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	store := memory.NewStore(database.SQLDB())
	if err := store.DeleteEntry(ctx, id); err != nil {
		return fmt.Errorf("delete entry: %w", err)
	}

	fmt.Printf("deleted entry %d\n", id)
	return nil
}

// cmdSession handles session subcommands like "start" and "end".
func cmdSession(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ilnamiqui session <start|end>")
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "start":
		return cmdSessionStart(subArgs)
	case "end":
		return cmdSessionEnd(subArgs)
	default:
		return fmt.Errorf("unknown session subcommand %q — use 'start' or 'end'", sub)
	}
}

// cmdSessionStart starts a new session for the project and prints the session ID.
func cmdSessionStart(args []string) error {
	fs := flag.NewFlagSet("session start", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	database, projectSlug, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	mgr := session.NewManager(database.SQLDB())
	sess, err := mgr.StartSession(ctx, projectSlug)
	if err != nil {
		return fmt.Errorf("start session: %w", err)
	}

	fmt.Println(sess.ID)
	return nil
}

func cmdSessionEnd(args []string) error {
	fs := flag.NewFlagSet("session end", flag.ContinueOnError)
	summary := fs.String("summary", "", "session summary")
	if err := fs.Parse(args); err != nil {
		return err
	}

	database, projectSlug, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	mgr := session.NewManager(database.SQLDB())
	sess, err := mgr.GetActiveSession(ctx, projectSlug)
	if err != nil {
		return fmt.Errorf("get active session: %w", err)
	}

	if err := mgr.EndSession(ctx, sess.ID, *summary); err != nil {
		return fmt.Errorf("end session: %w", err)
	}

	fmt.Printf("ended session %s\n", sess.ID)
	return nil
}

// cmdVersion prints the CLI version.
func cmdVersion() error {
	fmt.Println(version)
	return nil
}

// printHelp prints usage information for the CLI.
func printHelp() error {
	const help = `ilnamiqui — session memory for opencode (Nahuatl: "to remember")

Usage:
  ilnamiqui <command> [flags]

Commands:
  init                  Initialize database in .opencode/
  save <key> <value>    Save a memory entry
  load [--session] [--limit N]
                        Load memory entries (all or current session)
  list [--limit N]      List recent sessions
  search <query> [--limit N]
                        Search memory entries by key or value
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

// openDB finds the project root, opens the DB, and verifies migrations.
func openDB() (*db.DB, string, error) {
	dbPath, err := config.DBPath()
	if err != nil {
		return nil, "", fmt.Errorf("find db path: %w\n\nRun 'ilnamiqui init' first", err)
	}

	database, err := db.NewDB(dbPath)
	if err != nil {
		return nil, "", fmt.Errorf("open db: %w", err)
	}

	projectSlug, err := config.ProjectSlug()
	if err != nil {
		database.Close() //nolint:errcheck
		return nil, "", err
	}

	return database, projectSlug, nil
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
	fmt.Fprintln(w, "ID\tKey\tValue\tSession ID\tCreated")
	for _, e := range entries {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", e.ID, e.Key, e.Value, e.SessionID, e.CreatedAt.Format(time.RFC3339))
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
	fmt.Fprintln(w, "ID\tProject\tStarted\tEnded\tSummary")
	for _, s := range sessions {
		ended := ""
		if s.EndedAt != nil {
			ended = s.EndedAt.Format(time.RFC3339)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.ID, s.Project, s.StartedAt.Format(time.RFC3339), ended, s.Summary)
	}
	return w.Flush()
}
