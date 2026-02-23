package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/mydehq/autotitle"
	"github.com/mydehq/autotitle/internal/config"
	"github.com/mydehq/autotitle/internal/ui"
	"github.com/spf13/cobra"
)

var (
	flagInitURL       string
	flagInitFillerURL string
	flagInitForce     bool
	flagInitOffset    int
	flagInitSeparator string
	flagInitPadding   int
)

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Create a new _autotitle.yml map file",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		runInit(cmd, path)
	},
}

func init() {
	RootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&flagInitURL, "url", "u", "", "Provider URL (MAL, TMDB, etc)")
	initCmd.Flags().StringVarP(&flagInitFillerURL, "filler", "F", "", "Filler list URL")
	initCmd.Flags().BoolVarP(&flagInitForce, "force", "f", false, "Overwrite existing config")
	initCmd.Flags().IntVarP(&flagInitOffset, "offset", "o", 0, "Shift episode numbers (e.g. 12 to map Ep 1 to 13)")
	initCmd.Flags().StringVarP(&flagInitSeparator, "separator", "S", " ", "Output separator")
	initCmd.Flags().IntVarP(&flagInitPadding, "padding", "p", 0, "Episode number padding (e.g. 2 for 01)")
}

func runInit(cmd *cobra.Command, path string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		logger.Error("Failed to resolve path", "error", err)
		os.Exit(1)
	}

	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())

	// Non-interactive: --url provided OR not a TTY
	if flagInitURL == "" && !isTTY {
		logger.Error("URL required in non-interactive mode (use --url)")
		os.Exit(1)
	}

	if flagInitURL != "" || !isTTY {
		runInitNonInteractive(cmd, absPath)
		return
	}

	// Interactive path
	ui.ClearAndPrintBanner(flagDryRun)

	// Load defaults to find map file name
	defaults := config.GetDefaults()
	mapFileName := defaults.MapFile
	mapPath := filepath.Join(absPath, mapFileName)

	// Check for existing map file
	if _, err := os.Stat(mapPath); err == nil && !flagInitForce {
		overwrite := false
		err := ui.RunForm(huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Config already exists").
					Description(fmt.Sprintf("Overwrite %s?", ui.StylePath.Render(mapPath))).
					Value(&overwrite),
			),
		).WithTheme(ui.AutotitleTheme()))

		if err != nil {
			ui.HandleAbort(err)
			logger.Error("Init failed", "error", err)
			os.Exit(1)
		}

		if !overwrite {
			logger.Warn(ui.StyleDim.Render("Init cancelled"))
			return
		}
	}

	// Scan directory for patterns and media
	scanResult, err := config.Scan(absPath, defaults.Formats)
	if err != nil {
		logger.Error("Failed to scan directory", "error", err)
		os.Exit(1)
	}

	// Helper for safe flag access
	hasFlag := func(f string) bool {
		return cmd != nil && cmd.Flags().Lookup(f) != nil && cmd.Flags().Changed(f)
	}

	// Run the wizard
	flags := ui.InitFlags{
		URL:          flagInitURL,
		FillerURL:    flagInitFillerURL,
		HasFiller:    hasFlag("filler"),
		Separator:    flagInitSeparator,
		HasSeparator: hasFlag("separator"),
		Offset:       flagInitOffset,
		HasOffset:    hasFlag("offset"),
		Padding:      flagInitPadding,
		HasPadding:   hasFlag("padding"),
		DryRun:       flagDryRun,
	}

	ctx := context.Background()
	if cmd != nil {
		ctx = cmd.Context()
	}

	startRename, err := ui.RunInitWizard(ctx, absPath, scanResult, flags)
	if err != nil {
		logger.Error("Init failed", "error", err)
		os.Exit(1)
	}

	if startRename {
		logger.Info(ui.StyleHeader.Render("Starting renaming..."))
		runRename(ctx, cmd, absPath)
	}
}

// runInitNonInteractive handles the non-interactive init path using flag values.
func runInitNonInteractive(cmd *cobra.Command, absPath string) {
	opts := []autotitle.Option{
		autotitle.WithURL(flagInitURL),
		autotitle.WithFiller(flagInitFillerURL),
		autotitle.WithSeparator(flagInitSeparator),
		autotitle.WithOffset(flagInitOffset),
		autotitle.WithPadding(flagInitPadding),
	}

	if flagInitForce {
		opts = append(opts, autotitle.WithForce())
	}

	if err := autotitle.Init(cmd.Context(), absPath, opts...); err != nil {
		logger.Error("Failed to init config", "error", err)
		os.Exit(1)
	}

	defaults := config.GetDefaults()
	mapFile := defaults.MapFile
	logger.Success(fmt.Sprintf("%s: %s", ui.StyleHeader.Render("Created config"), ui.StylePath.Render(filepath.Join(absPath, mapFile))))
}
