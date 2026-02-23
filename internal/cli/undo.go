package cli

import (
	"fmt"
	"os"

	"github.com/mydehq/autotitle"
	"github.com/mydehq/autotitle/internal/ui"
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
		fmt.Println()
		logger.Error("Failed to undo", "error", err)
		os.Exit(1)
	}
	fmt.Println()
	logger.Success(ui.StyleHeader.Render("Files restored from backup"))
}
