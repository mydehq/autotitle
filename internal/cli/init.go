package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mydehq/autotitle"
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
	initCmd.Flags().IntVarP(&flagInitOffset, "offset", "o", 0, "Episode number offset")
	initCmd.Flags().StringVarP(&flagInitSeparator, "separator", "S", " ", "Output separator")
	initCmd.Flags().IntVarP(&flagInitPadding, "padding", "p", 0, "Episode number padding (e.g. 2 for 01)")
}

func runInit(cmd *cobra.Command, path string) {
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

	if err := autotitle.Init(cmd.Context(), path, opts...); err != nil {
		logger.Error(fmt.Sprintf("Failed to init config: %v", err))
		os.Exit(1)
	}

	mapFile := "_autotitle.yml"
	logger.Info(StyleHeader.Render("Created config"), "path", StylePath.Render(filepath.Join(path, mapFile)))
}
