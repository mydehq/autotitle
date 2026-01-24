package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mydehq/autotitle/internal/api"
	"github.com/mydehq/autotitle/internal/types"
	"github.com/mydehq/autotitle/internal/version"

	"github.com/spf13/cobra"
)

var (
	flagDryRun    bool
	flagNoBackup  bool
	flagVerbose   bool
	flagQuiet     bool
	flagForce     bool
	flagAll       bool
	flagGlobal    bool
	flagProvider  string
	flagFillerURL string
)

func main() {
	ctx := context.Background()

	rootCmd := &cobra.Command{
		Use:   "autotitle <path>",
		Short: "Rename anime episodes with proper titles",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runRename(ctx, args[0])
		},
	}

	// Rename flags
	rootCmd.Flags().BoolVarP(&flagDryRun, "dry-run", "d", false, "Preview changes without applying")
	rootCmd.Flags().BoolVarP(&flagNoBackup, "no-backup", "n", false, "Skip backup creation")
	rootCmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Verbose output")
	rootCmd.Flags().BoolVarP(&flagQuiet, "quiet", "q", false, "Quiet mode")

	// Init command
	initCmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Create a new _autotitle.yml map file",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}
			runInit(ctx, path)
		},
	}
	initCmd.Flags().StringVarP(&flagFillerURL, "filler", "F", "", "Filler list URL")
	initCmd.Flags().BoolVarP(&flagForce, "force", "f", false, "Overwrite existing config")

	// DB commands
	dbCmd := &cobra.Command{
		Use:   "db",
		Short: "Database management commands",
	}

	dbGenCmd := &cobra.Command{
		Use:   "gen <url>",
		Short: "Generate episode database from URL",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runDBGen(ctx, args[0])
		},
	}
	dbGenCmd.Flags().StringVarP(&flagFillerURL, "filler", "F", "", "Filler list URL")
	dbGenCmd.Flags().BoolVarP(&flagForce, "force", "f", false, "Overwrite existing database")

	dbListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all cached databases",
		Run: func(cmd *cobra.Command, args []string) {
			runDBList(ctx)
		},
	}
	dbListCmd.Flags().StringVarP(&flagProvider, "provider", "p", "", "Filter by provider (mal, tmdb, etc)")

	dbInfoCmd := &cobra.Command{
		Use:   "info <provider> <id>",
		Short: "Show database info",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			runDBInfo(ctx, args[0], args[1])
		},
	}

	dbRmCmd := &cobra.Command{
		Use:   "rm <provider> <id>",
		Short: "Remove a database",
		Args:  cobra.MaximumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			runDBRm(ctx, args)
		},
	}
	dbRmCmd.Flags().BoolVarP(&flagAll, "all", "a", false, "Remove all databases")

	dbPathCmd := &cobra.Command{
		Use:   "path",
		Short: "Show database directory path",
		Run: func(cmd *cobra.Command, args []string) {
			runDBPath()
		},
	}

	dbCmd.AddCommand(dbGenCmd, dbListCmd, dbInfoCmd, dbRmCmd, dbPathCmd)

	// Undo command
	undoCmd := &cobra.Command{
		Use:   "undo <path>",
		Short: "Restore files from backup",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runUndo(ctx, args[0])
		},
	}

	// Clean command
	cleanCmd := &cobra.Command{
		Use:   "clean [path]",
		Short: "Remove backup directory (-g for all backups globally)",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runClean(ctx, args)
		},
	}
	cleanCmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Remove all backups globally")

	// Version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("autotitle %s\n", version.String())
		},
	}

	rootCmd.AddCommand(initCmd, dbCmd, undoCmd, cleanCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runRename(ctx context.Context, path string) {
	var opts []api.Option

	if flagDryRun {
		opts = append(opts, api.WithDryRun())
	}
	if flagNoBackup {
		opts = append(opts, api.WithNoBackup())
	}
	if !flagQuiet {
		opts = append(opts, api.WithEvents(func(e types.Event) {
			switch e.Type {
			case types.EventSuccess:
				fmt.Printf("✓ %s\n", e.Message)
			case types.EventWarning:
				fmt.Printf("⚠ %s\n", e.Message)
			case types.EventError:
				fmt.Printf("✗ %s\n", e.Message)
			default:
				if flagVerbose {
					fmt.Println(e.Message)
				}
			}
		}))
	}

	ops, err := api.Rename(ctx, path, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Summary
	var success, skipped, failed int
	for _, op := range ops {
		switch op.Status {
		case types.StatusSuccess:
			success++
		case types.StatusSkipped:
			skipped++
		case types.StatusFailed:
			failed++
		}
	}

	if !flagQuiet {
		fmt.Printf("\nRenamed: %d, Skipped: %d, Failed: %d\n", success, skipped, failed)
	}
}

func runInit(ctx context.Context, path string) {
	if err := api.Init(ctx, path, "", flagFillerURL, flagForce); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created %s/_autotitle.yml\n", path)
}

func runDBGen(ctx context.Context, url string) {
	generated, err := api.DBGen(ctx, url, flagFillerURL, flagForce)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if generated {
		fmt.Println("Database generated successfully")
	} else {
		fmt.Println("Database already exists (cached)")
	}
}

func runDBList(ctx context.Context) {
	items, err := api.DBList(ctx, flagProvider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(items) == 0 {
		fmt.Println("No databases found")
		return
	}

	fmt.Println("Cached databases:")
	for _, item := range items {
		fmt.Printf("  %s/%s: %s (%d episodes)\n", item.Provider, item.ID, item.Title, item.EpisodeCount)
	}
}

func runDBInfo(ctx context.Context, prov, id string) {
	media, err := api.DBInfo(ctx, prov, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if media == nil {
		fmt.Fprintln(os.Stderr, "Error: Database not found")
		os.Exit(1)
	}

	fmt.Printf("Provider: %s\n", media.Provider)
	fmt.Printf("ID: %s\n", media.ID)
	fmt.Printf("Title: %s\n", media.Title)
	fmt.Printf("Episodes: %d\n", len(media.Episodes))
	if media.FillerSource != "" {
		fmt.Printf("Filler Source: %s\n", media.FillerSource)
	}
}

func runDBRm(ctx context.Context, args []string) {
	if flagAll {
		if err := api.DBDeleteAll(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Deleted all databases")
		return
	}

	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: autotitle db rm <provider> <id>")
		os.Exit(1)
	}

	if err := api.DBDelete(ctx, args[0], args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Deleted database: %s/%s\n", args[0], args[1])
}

func runDBPath() {
	path, err := api.DBPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(path)
}

func runUndo(ctx context.Context, path string) {
	if err := api.Undo(ctx, path); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Files restored from backup")
}

func runClean(ctx context.Context, args []string) {
	if flagGlobal {
		if err := api.CleanAll(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Removed all backups globally")
		return
	}

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Please specify a path or use -g for global cleanup")
		os.Exit(1)
	}

	if err := api.Clean(ctx, args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Removed backup for: %s\n", args[0])
}
