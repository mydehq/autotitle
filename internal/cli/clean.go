package cli

import (
	"fmt"
	"os"

	"github.com/mydehq/autotitle"
	"github.com/mydehq/autotitle/internal/ui"
	"github.com/spf13/cobra"
)

var flagCleanAll bool

var cleanCmd = &cobra.Command{
	Use:   "clean [path]",
	Short: "Remove backup directory (-a for all backups globally)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runClean(cmd, args)
	},
}

func init() {
	RootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVarP(&flagCleanAll, "all", "a", false, "Remove all backups globally")
}

func runClean(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()
	if flagCleanAll {
		if err := autotitle.CleanAll(ctx); err != nil {
			logger.Error("Failed to clean global backups", "error", err)
			os.Exit(1)
		}
		logger.Success(ui.StyleHeader.Render("Removed all backups globally"))
		return
	}

	if len(args) == 0 {
		logger.Error("Please specify a path or use -a for global cleanup")
		os.Exit(1)
	}

	if err := autotitle.Clean(ctx, args[0]); err != nil {
		logger.Error("Failed to remove backup", "path", args[0], "error", err)
		os.Exit(1)
	}
	logger.Success(fmt.Sprintf("%s: %s", ui.StyleHeader.Render("Removed backup"), ui.StylePath.Render(args[0])))
}
