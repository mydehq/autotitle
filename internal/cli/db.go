package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

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
	Use:   "info <provider>/<id>",
	Short: "Show database info",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runDBInfo(cmd.Context(), args[0])
	},
}

var dbRmCmd = &cobra.Command{
	Use:   "rm <provider>/<id>",
	Short: "Remove a database",
	Args:  cobra.MaximumNArgs(1),
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
		logger.Info(StyleHeader.Render("Database generated"), "url", StylePath.Render(url))
	} else {
		logger.Info(StyleHeader.Render("Database cached"), "url", StylePath.Render(url))
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

	fmt.Printf("%s: %s\n", StyleHeader.Render("Cached databases"), StylePattern.Render(fmt.Sprint(len(items))))
	for _, item := range items {
		fmt.Printf("  %s %s/%s: %s %s\n",
			StyleDim.Render("-"),
			StyleHeader.Render(item.Provider),
			StylePath.Render(item.ID),
			item.Title,
			StyleDim.Render(fmt.Sprintf("(%d episodes)", item.EpisodeCount)),
		)
	}
}

func runDBInfo(ctx context.Context, target string) {
	parts := strings.Split(target, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		logger.Error("Invalid format. Use: <provider>/<id> (e.g. mal/269)")
		os.Exit(1)
	}
	prov, id := parts[0], parts[1]

	media, err := autotitle.DBInfo(ctx, prov, id)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get database info: %v", err))
		os.Exit(1)
	}
	if media == nil {
		logger.Error("Database not found")
		os.Exit(1)
	}

	keyStyle := StyleHeader.Width(15)

	fmt.Printf("%s %s\n", keyStyle.Render("Title:"), media.Title)
	fmt.Printf("%s %d\n", keyStyle.Render("Episodes:"), len(media.Episodes))
	fmt.Printf("%s %s\n", keyStyle.Render("ID:"), StylePath.Render(media.ID))
	fmt.Printf("%s %s\n", keyStyle.Render("Provider:"), StylePattern.Render(media.Provider))
	if media.FillerSource != "" {
		fmt.Printf("%s %s\n", keyStyle.Render("Filler Source:"), media.FillerSource)
	}
}

func runDBRm(ctx context.Context, args []string) {
	if flagDBAll {
		if err := autotitle.DBDeleteAll(ctx); err != nil {
			logger.Error(fmt.Sprintf("Failed to delete all databases: %v", err))
			os.Exit(1)
		}
		logger.Info(StyleHeader.Render("Deleted all databases"))
		return
	}

	if len(args) == 0 {
		logger.Error("Usage: autotitle db rm <provider>/<id>")
		os.Exit(1)
	}

	parts := strings.Split(args[0], "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		logger.Error("Invalid format. Use: <provider>/<id> (e.g. mal/269)")
		os.Exit(1)
	}
	prov, id := parts[0], parts[1]

	if err := autotitle.DBDelete(ctx, prov, id); err != nil {
		logger.Error(fmt.Sprintf("Failed to delete database: %v", err))
		os.Exit(1)
	}
	logger.Info(StyleHeader.Render("Deleted database"), "provider", prov, "id", StylePath.Render(id))
}

func runDBPath() {
	path, err := autotitle.DBPath()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get DB path: %v", err))
		os.Exit(1)
	}
	fmt.Println(path)
}
