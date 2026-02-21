package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/mydehq/autotitle"
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
	RootCmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Verbose output")
	RootCmd.Flags().IntVarP(&flagOffset, "offset", "o", 0, "Episode number offset (db_num = local_num + offset)")
	RootCmd.Flags().StringVarP(&flagFillerURL, "filler", "F", "", "Override filler source URL")
	RootCmd.Flags().BoolVarP(&flagForce, "force", "f", false, "Force database refresh")
	RootCmd.Flags().BoolVarP(&flagNoTag, "no-tag", "T", false, "Disable MKV metadata tagging (mkvpropedit)")
	RootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress output except errors")

	// Default logger setup (before flags parse)
	logger = log.New(os.Stdout)
	configureStyles()

	autotitle.SetDefaultEventHandler(func(e autotitle.Event) {
		switch e.Type {
		case autotitle.EventSuccess:
			logger.Info(e.Message)
		case autotitle.EventWarning:
			logger.Warn(e.Message)
		case autotitle.EventError:
			logger.Error(e.Message)
		default:
			logger.Debug(e.Message)
		}
	})

	colorizeHelp(RootCmd)
}

func configureStyles() {
	styles := log.DefaultStyles()

	styles.Levels[log.DebugLevel] = lipgloss.NewStyle().
		SetString("DEBUG").
		Bold(true).
		Foreground(lipgloss.Color("63"))

	styles.Levels[log.InfoLevel] = lipgloss.NewStyle().
		SetString("INFO ").
		Bold(true).
		Foreground(lipgloss.Color("86"))

	styles.Levels[log.WarnLevel] = lipgloss.NewStyle().
		SetString("WARN ").
		Bold(true).
		Foreground(lipgloss.Color("192"))

	styles.Levels[log.ErrorLevel] = lipgloss.NewStyle().
		SetString("ERROR").
		Bold(true).
		Foreground(lipgloss.Color("204"))

	logger.SetStyles(styles)
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
		logger.Info("Summary",
			"renamed", lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(fmt.Sprint(success)),
			"skipped", lipgloss.NewStyle().Foreground(lipgloss.Color("192")).Render(fmt.Sprint(skipped)),
			"failed", lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Render(fmt.Sprint(failed)),
		)
	}
}
