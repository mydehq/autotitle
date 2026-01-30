package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/mydehq/autotitle"
	"github.com/spf13/cobra"
)

var (
	flagDBFillerURL string
	flagDBForce     bool
	flagDBProvider  string
	flagDBAll       bool
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management commands",
}

var dbGenCmd = &cobra.Command{
	Use:   "gen <url>",
	Short: "Generate episode database from URL",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runDBGen(cmd.Context(), args[0])
	},
}

var dbListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cached databases",
	Run: func(cmd *cobra.Command, args []string) {
		runDBList(cmd.Context())
	},
}

var dbInfoCmd = &cobra.Command{
	Use:   "info <provider> <id>",
	Short: "Show database info",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		runDBInfo(cmd.Context(), args[0], args[1])
	},
}

var dbRmCmd = &cobra.Command{
	Use:   "rm <provider> <id>",
	Short: "Remove a database",
	Args:  cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		runDBRm(cmd.Context(), args)
	},
}

var dbPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show database directory path",
	Run: func(cmd *cobra.Command, args []string) {
		runDBPath()
	},
}

func init() {
	RootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(dbGenCmd, dbListCmd, dbInfoCmd, dbRmCmd, dbPathCmd)

	dbGenCmd.Flags().StringVarP(&flagDBFillerURL, "filler", "F", "", "Filler list URL")
	dbGenCmd.Flags().BoolVarP(&flagDBForce, "force", "f", false, "Overwrite existing database")
	dbListCmd.Flags().StringVarP(&flagDBProvider, "provider", "p", "", "Filter by provider (mal, tmdb, etc)")
	dbRmCmd.Flags().BoolVarP(&flagDBAll, "all", "a", false, "Remove all databases")
}

func runDBGen(ctx context.Context, url string) {
	opts := []autotitle.Option{}

	if flagDBFillerURL != "" {
		opts = append(opts, autotitle.WithFiller(flagDBFillerURL))
	}

	if flagDBForce {
		opts = append(opts, autotitle.WithForce())
	}

	generated, err := autotitle.DBGen(ctx, url, opts...)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to generate database: %v", err))
		os.Exit(1)
	}

	if generated {
		logger.Info("Database generated", "url", url)
	} else {
		logger.Info("Database cached", "url", url)
	}
}

func runDBList(ctx context.Context) {
	items, err := autotitle.DBList(ctx, flagDBProvider)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to list databases: %v", err))
		os.Exit(1)
	}

	if len(items) == 0 {
		logger.Info("No databases found")
		return
	}

	logger.Info("Cached databases", "count", len(items))
	for _, item := range items {
		fmt.Printf("  %s/%s: %s (%d episodes)\n", item.Provider, item.ID, item.Title, item.EpisodeCount)
	}
}

func runDBInfo(ctx context.Context, prov, id string) {
	media, err := autotitle.DBInfo(ctx, prov, id)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get database info: %v", err))
		os.Exit(1)
	}
	if media == nil {
		logger.Error("Database not found")
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
	if flagDBAll {
		if err := autotitle.DBDeleteAll(ctx); err != nil {
			logger.Error(fmt.Sprintf("Failed to delete all databases: %v", err))
			os.Exit(1)
		}
		logger.Info("Deleted all databases")
		return
	}

	if len(args) < 2 {
		logger.Error("Usage: autotitle db rm <provider> <id>")
		os.Exit(1)
	}

	if err := autotitle.DBDelete(ctx, args[0], args[1]); err != nil {
		logger.Error(fmt.Sprintf("Failed to delete database: %v", err))
		os.Exit(1)
	}
	logger.Info("Deleted database", "provider", args[0], "id", args[1])
}

func runDBPath() {
	path, err := autotitle.DBPath()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get DB path: %v", err))
		os.Exit(1)
	}
	fmt.Println(path)
}
