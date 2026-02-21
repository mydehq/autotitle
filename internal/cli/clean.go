package cli

import (
	"fmt"
	"os"

	"github.com/mydehq/autotitle"
	"github.com/spf13/cobra"
)

var flagCleanGlobal bool

var cleanCmd = &cobra.Command{
	Use:   "clean [path]",
	Short: "Remove backup directory (-g for all backups globally)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runClean(cmd, args)
	},
}

func init() {
	RootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVarP(&flagCleanGlobal, "global", "g", false, "Remove all backups globally")
}

func runClean(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()
	if flagCleanGlobal {
		if err := autotitle.CleanAll(ctx); err != nil {
			logger.Error(fmt.Sprintf("Failed to clean global backups: %v", err))
			os.Exit(1)
		}
		logger.Info(StyleHeader.Render("Removed all backups globally"))
		return
	}

	if len(args) == 0 {
		logger.Error("Please specify a path or use -g for global cleanup")
		os.Exit(1)
	}

	if err := autotitle.Clean(ctx, args[0]); err != nil {
		logger.Error(fmt.Sprintf("Failed to remove backup for %s: %v", args[0], err))
		os.Exit(1)
	}
	logger.Info(StyleHeader.Render("Removed backup"), "path", StylePath.Render(args[0]))
}
