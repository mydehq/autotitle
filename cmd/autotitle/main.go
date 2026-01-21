package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/soymadip/autotitle/internal/api"
	"github.com/soymadip/autotitle/internal/logger"

	"github.com/spf13/cobra"
)

var (
	// Flags
	flagDryRun   bool
	flagNoBackup bool
	flagVerbose  bool
	flagQuiet    bool
	flagConfig   string
	flagOutput   string

	flagAnime     string
	flagFiller    string
	flagForce     bool
	flagAll       bool
	flagRateLimit int

	Version = "v0.1.0"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "autotitle <path>",
		Short: "Rename anime episodes with proper titles",
		Args:  cobra.ExactArgs(1),
		Run:   runRenamer,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&flagConfig, "config", "c", "", "Custom configuration file path")

	// Rename flags
	rootCmd.Flags().BoolVarP(&flagDryRun, "dry-run", "d", false, "Preview changes without applying")
	rootCmd.Flags().BoolVarP(&flagNoBackup, "no-backup", "n", false, "Skip backup creation")
	rootCmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Verbose output")
	rootCmd.Flags().BoolVarP(&flagQuiet, "quiet", "q", false, "Quiet mode")

	// Init command
	initCmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Create a new _autotitle.yml config file",
		Args:  cobra.MaximumNArgs(1),
		Run:   runInit,
	}
	initCmd.Flags().StringVarP(&flagAnime, "anime", "a", "", "Anime series name or MAL URL")
	initCmd.Flags().StringVarP(&flagFiller, "filler", "F", "", "AnimeFillerList URL or slug")
	initCmd.Flags().BoolVarP(&flagForce, "force", "f", false, "Overwrite existing config")

	// DB commands
	dbCmd := &cobra.Command{
		Use:   "db",
		Short: "Database management commands",
	}

	dbGenCmd := &cobra.Command{
		Use:   "gen <mal_url>",
		Short: "Generate episode database from MAL",
		Args:  cobra.ExactArgs(1),
		Run:   runDBGen,
	}
	dbGenCmd.Flags().StringVarP(&flagFiller, "filler", "F", "", "AnimeFillerList URL or slug")
	dbGenCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output directory")
	dbGenCmd.Flags().BoolVarP(&flagForce, "force", "f", false, "Overwrite existing database")
	dbGenCmd.Flags().IntVarP(&flagRateLimit, "rate-limit", "r", 0, "API rate limit (requests per second)")

	dbListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all cached databases",
		Run:   runDBList,
	}

	dbInfoCmd := &cobra.Command{
		Use:   "info <series_id>",
		Short: "Show database info",
		Args:  cobra.ExactArgs(1),
		Run:   runDBInfo,
	}

	dbRmCmd := &cobra.Command{
		Use:   "rm <series_id>",
		Short: "Remove a database",
		Args:  cobra.MaximumNArgs(1),
		Run:   runDBRm,
	}
	dbRmCmd.Flags().BoolVarP(&flagAll, "all", "a", false, "Remove all databases")

	dbPathCmd := &cobra.Command{
		Use:   "path",
		Short: "Show database directory path",
		Run:   runDBPath,
	}

	dbCmd.AddCommand(dbGenCmd, dbListCmd, dbInfoCmd, dbRmCmd, dbPathCmd)

	// Undo command
	undoCmd := &cobra.Command{
		Use:   "undo <path>",
		Short: "Restore files from backup",
		Args:  cobra.ExactArgs(1),
		Run:   runUndo,
	}

	// Clean command
	cleanCmd := &cobra.Command{
		Use:   "clean <path>",
		Short: "Remove backup directory",
		Args:  cobra.ExactArgs(1),
		Run:   runClean,
	}

	// Version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of autotitle",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("autotitle %s\n", Version)
		},
	}

	rootCmd.AddCommand(initCmd, dbCmd, undoCmd, cleanCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runRenamer(cmd *cobra.Command, args []string) {
	opts := []api.Option{
		api.WithConfig(flagConfig),
	}
	if flagDryRun {
		opts = append(opts, api.WithDryRun())
	}
	if flagNoBackup {
		opts = append(opts, api.WithNoBackup())
	}
	if flagVerbose {
		opts = append(opts, api.WithVerbose())
	}
	if flagQuiet {
		opts = append(opts, api.WithQuiet())
	}

	if err := api.Rename(args[0], opts...); err != nil {
		logger.Fatal("Rename failed: %v", err)
	}
}

func runInit(cmd *cobra.Command, args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	opts := []api.Option{
		api.WithConfig(flagConfig),
	}
	if flagForce {
		opts = append(opts, api.WithForce())
	}
	if flagAnime != "" {
		opts = append(opts, api.WithAnime(flagAnime))
	}
	if flagFiller != "" {
		opts = append(opts, api.WithFiller(flagFiller))
	}

	if err := api.Init(path, opts...); err != nil {
		logger.Fatal("Init failed: %v", err)
	}

	configPath := filepath.Join(path, "_autotitle.yml")
	logger.Success("Created %s", configPath)
}

func runDBGen(cmd *cobra.Command, args []string) {
	malURL := args[0]

	opts := func(o *api.DBGenOptions) {
		o.MALURL = malURL
		o.AFLURL = flagFiller
		o.OutputDir = flagOutput
		o.Force = flagForce
		o.RateLimit = flagRateLimit
		o.ConfigPath = flagConfig
	}

	if err := api.DBGen(malURL, opts); err != nil {
		logger.Fatal("DBGen failed: %v", err)
	}

	// Extract MAL ID and load database info for success message
	malID := api.ExtractMALID(malURL)
	if malID > 0 {
		seriesID := fmt.Sprintf("%d", malID)
		if sd, err := api.DBInfo(seriesID, flagOutput); err == nil {
			logger.Success("Database generated: %s (%d episodes)", sd.Title, sd.EpisodeCount)
			return
		}
	}

	if flagVerbose {
		logger.Success("Database generation completed")
	}
}

func runDBList(cmd *cobra.Command, args []string) {
	ids, err := api.DBList(flagOutput)
	if err != nil {
		logger.Fatal("Failed to list databases: %v", err)
	}

	if len(ids) == 0 {
		logger.Info("No databases found")
		return
	}

	logger.Info("Cached databases:")
	for _, id := range ids {
		sd, err := api.DBInfo(id, flagOutput)
		if err != nil {
			fmt.Printf("  - %s (error loading)\n", id)
			continue
		}
		fmt.Printf("  - %s: %s (%d episodes)\n", sd.MALID, sd.Title, sd.EpisodeCount)
	}
}

func runDBInfo(cmd *cobra.Command, args []string) {
	query := args[0]
	matches, err := api.FindSeriesByQuery(query, flagOutput)
	if err != nil {
		logger.Fatal("Error finding series: %v", err)
	}

	if len(matches) == 0 {
		logger.Fatal("No database found for query: %s", query)
	}

	var match api.Match
	if len(matches) > 1 {
		logger.Info("Multiple matches found, please select one:")
		for i, m := range matches {
			fmt.Printf("%d: %s (%s)\n", i+1, m.Title, m.MALID)
		}
		// Simple selection for now, can be improved with interactive prompt
		logger.Fatal("Ambiguous query, please be more specific or use MAL ID.")
	} else {
		match = matches[0]
	}

	sd, err := api.DBInfo(match.MALID, flagOutput)
	if err != nil {
		logger.Fatal("Failed to load database: %v", err)
	}

	logger.Info("Series ID: %s", sd.MALID)
	logger.Info("Title: %s", sd.Title)
	logger.Info("Last Update: %s", sd.LastUpdate.Format("2006-01-02 15:04:05"))
	logger.Info("Episodes: %d", sd.EpisodeCount)
}

func runDBRm(cmd *cobra.Command, args []string) {
	if flagAll {
		if err := api.DBRm("", flagOutput, true); err != nil {
			logger.Fatal("Failed to delete all databases: %v", err)
		}
		logger.Success("Deleted all databases")
		return
	}

	if len(args) == 0 {
		logger.Fatal("No series ID, URL, or query provided.")
	}

	query := args[0]
	matches, err := api.FindSeriesByQuery(query, flagOutput)
	if err != nil {
		logger.Fatal("Error finding series: %v", err)
	}

	if len(matches) == 0 {
		logger.Fatal("No database found for query: %s", query)
	}

	var match api.Match
	if len(matches) > 1 {
		logger.Info("Multiple matches found, please select one:")
		for i, m := range matches {
			fmt.Printf("%d: %s (%s)\n", i+1, m.Title, m.MALID)
		}
		logger.Fatal("Ambiguous query, please be more specific or use MAL ID.")
	} else {
		match = matches[0]
	}

	if err := api.DBRm(match.MALID, flagOutput, false); err != nil {
		logger.Fatal("Failed to delete database: %v", err)
	}

	logger.Success("Deleted database: %s (%s)", match.Title, match.MALID)
}

func runDBPath(cmd *cobra.Command, args []string) {
	path, err := api.DBPath(flagOutput)
	if err != nil {
		logger.Fatal("Failed to get database path: %v", err)
	}
	fmt.Println(path)
}

func runUndo(cmd *cobra.Command, args []string) {
	if err := api.Undo(args[0]); err != nil {
		logger.Fatal("Undo failed: %v", err)
	}
}

func runClean(cmd *cobra.Command, args []string) {
	if err := api.Clean(args[0]); err != nil {
		logger.Fatal("Clean failed: %v", err)
	}
}
