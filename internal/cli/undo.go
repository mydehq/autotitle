package cli

import (
	"fmt"
	"os"

	"github.com/mydehq/autotitle"
	"github.com/spf13/cobra"
)

var undoCmd = &cobra.Command{
	Use:   "undo <path>",
	Short: "Restore files from backup",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runUndo(cmd, args[0])
	},
}

func init() {
	RootCmd.AddCommand(undoCmd)
}

func runUndo(cmd *cobra.Command, path string) {
	if err := autotitle.Undo(cmd.Context(), path); err != nil {
		logger.Error(fmt.Sprintf("Failed to undo: %v", err))
		os.Exit(1)
	}
	logger.Info(StyleHeader.Render("Files restored from backup"))
}
