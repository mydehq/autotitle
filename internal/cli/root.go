package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/mydehq/autotitle"
	"github.com/mydehq/autotitle/internal/version"
	"github.com/spf13/cobra"
)

var (
	flagDryRun    bool
	flagNoBackup  bool
	flagVerbose   bool
	flagQuiet     bool
	flagNoTag     bool
	flagOffset    int
	flagFillerURL string
	flagForce     bool

	logger *log.Logger
)

var RootCmd = &cobra.Command{
	Use:           "autotitle <path>",
	Short:         "Rename media files with proper titles",
	Version:       version.String(),
	SilenceErrors: true,
	SilenceUsage:  true,
	Args:          cobra.ExactArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogger()
	},
	Run: func(cmd *cobra.Command, args []string) {
		runRename(cmd.Context(), cmd, args[0])
	},
}

func Execute() {
	fmt.Println()
	if err := RootCmd.Execute(); err != nil {
		if logger != nil {
			logger.Error(err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		RootCmd.Usage()
		os.Exit(1)
	}
}

func init() {
	RootCmd.Flags().BoolVarP(&flagDryRun, "dry-run", "d", false, "Preview changes without applying")
	RootCmd.Flags().BoolVarP(&flagNoBackup, "no-backup", "n", false, "Skip backup creation")
	RootCmd.Flags().BoolVarP(&flagVerbose, "verbose", "V", false, "Verbose output")
	RootCmd.Flags().IntVarP(&flagOffset, "offset", "o", 0, "Episode number offset (db_num = local_num + offset)")
	RootCmd.Flags().StringVarP(&flagFillerURL, "filler", "F", "", "Override filler source URL")
	RootCmd.Flags().BoolVarP(&flagForce, "force", "f", false, "Force database refresh")
	RootCmd.Flags().BoolVarP(&flagNoTag, "no-tag", "T", false, "Disable MKV metadata tagging (mkvpropedit)")
	RootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress output except errors")

	// Default logger setup (before flags parse)
	logger = log.New(os.Stdout)
	configureStyles()

	autotitle.SetDefaultEventHandler(func(e autotitle.Event) {
		msg := colorizeEvent(e.Message)
		switch e.Type {
		case autotitle.EventSuccess:
			logger.Info(msg)
		case autotitle.EventWarning:
			logger.Warn(msg)
		case autotitle.EventError:
			logger.Error(msg)
		default:
			logger.Debug(msg)
		}
	})

	colorizeHelp(RootCmd)

	// Pre-register version flag with -v shorthand.
	RootCmd.Flags().BoolP("version", "v", false, "Print version information")
}

func setupLogger() {
	if flagQuiet {
		logger.SetLevel(log.ErrorLevel)
	} else if flagVerbose {
		logger.SetLevel(log.DebugLevel)
	} else {
		logger.SetLevel(log.InfoLevel)
	}
}

func runRename(ctx context.Context, cmd *cobra.Command, path string) {
	var opts []autotitle.Option

	if flagDryRun {
		opts = append(opts, autotitle.WithDryRun())
	}

	if flagNoBackup {
		opts = append(opts, autotitle.WithNoBackup())
	}

	if cmd.Flags().Changed("offset") {
		opts = append(opts, autotitle.WithOffset(flagOffset))
	}

	if flagFillerURL != "" {
		opts = append(opts, autotitle.WithFiller(flagFillerURL))
	}
	if flagForce {
		opts = append(opts, autotitle.WithForce())
	}

	if !flagQuiet {
		// No need to pass events manually anymore, global default is used
	}

	ops, err := autotitle.Rename(ctx, path, opts...)
	if err != nil {
		logger.Error("Operation failed", "error", err)
		os.Exit(1)
	}

	// Summary
	var success, skipped, failed int

	for _, op := range ops {
		switch op.Status {
		case autotitle.StatusSuccess:
			success++
		case autotitle.StatusSkipped:
			skipped++
		case autotitle.StatusFailed:
			failed++
		}
	}

	if !flagQuiet {
		fmt.Println()
		logger.Info(fmt.Sprintf("Summary: renamed=%s skipped=%s failed=%s",
			StyleCommand.Render(fmt.Sprint(success)),
			StylePattern.Render(fmt.Sprint(skipped)),
			styleFlag.Render(fmt.Sprint(failed)),
		))
	}
}
