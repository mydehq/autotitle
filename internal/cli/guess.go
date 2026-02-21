package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mydehq/autotitle/internal/config"
	"github.com/spf13/cobra"
)

var guessPatternCmd = &cobra.Command{
	Use:   "guess-pattern [path]",
	Short: "Scan a directory and output detected patterns",
	Long:  "Scans the specified directory for media files and prints the unique patterns detected by the library's guesser.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		runGuessPattern(cmd, path)
	},
}

func init() {
	RootCmd.AddCommand(guessPatternCmd)
}

func runGuessPattern(cmd *cobra.Command, path string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to resolve path: %v", err))
		os.Exit(1)
	}

	// Load global config to get supported formats
	globalCfg, _ := config.LoadGlobal()
	defaults := config.GetDefaults()
	formats := defaults.Formats
	if globalCfg != nil && len(globalCfg.Formats) > 0 {
		formats = globalCfg.Formats
	}

	scanResult, err := config.Scan(absPath, formats)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to scan directory: %v", err))
		os.Exit(1)
	}

	if !scanResult.HasMedia {
		fmt.Printf("No media files found in: %s\n", StylePath.Render(absPath))
		return
	}

	fmt.Printf("%s in: %s\n", StyleHeader.Render("Detected patterns"), StylePath.Render(absPath))
	for _, p := range scanResult.DetectedPatterns {
		fmt.Printf(" %s %s\n", StyleDim.Render("-"), StylePattern.Render(p))
	}
}
