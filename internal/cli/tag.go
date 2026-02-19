package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mydehq/autotitle"
	"github.com/mydehq/autotitle/internal/tagger"
	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag [path]",
	Short: "Embed metadata into MKV files without renaming",
	Long: `tag reads the local _autotitle.yml and embeds episode/series metadata
into matched MKV files using mkvpropedit (MKVToolNix).

Useful for files that are already correctly named.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			logger.Error(fmt.Sprintf("Invalid path: %v", err))
			os.Exit(1)
		}
		runTag(cmd, absPath)
	},
}

func init() {
	RootCmd.AddCommand(tagCmd)
}

func runTag(cmd *cobra.Command, path string) {
	if !tagger.IsAvailable() {
		logger.Error("mkvpropedit not found. Please install MKVToolNix.")
		os.Exit(1)
	}

	opts := []autotitle.Option{
		autotitle.WithEvents(func(e autotitle.Event) {
			switch e.Type {
			case autotitle.EventInfo:
				logger.Info(e.Message)
			case autotitle.EventSuccess:
				logger.Info(e.Message)
			case autotitle.EventWarning:
				logger.Warn(e.Message)
			case autotitle.EventError:
				logger.Error(e.Message)
			}
		}),
	}

	if err := autotitle.Tag(cmd.Context(), path, opts...); err != nil {
		logger.Error(fmt.Sprintf("Tagging failed: %v", err))
		os.Exit(1)
	}
}
