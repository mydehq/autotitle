package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/mydehq/autotitle"
	"github.com/mydehq/autotitle/internal/types"
	"github.com/mydehq/autotitle/internal/ui"
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

	logger *ui.Logger
)

var RootCmd = &cobra.Command{
	Use:           "autotitle <path>",
	Short:         "Rename media files with proper titles",
	Version:       version.String(),
	SilenceErrors: true,
	SilenceUsage:  true,
	Args:          cobra.MaximumNArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogger()
	},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			logger.Error("No path provided.\n")
			fmt.Println(ui.StyleHeader.Render("Try running:"))
			fmt.Printf("    %s %s\n", ui.StyleCommand.Render("autotitle ."), ui.StyleDim.Render("  Process current directory"))
			fmt.Printf("    %s %s\n", ui.StyleCommand.Render("autotitle -h"), ui.StyleDim.Render(" Show all commands and flags"))
			fmt.Println()
			os.Exit(1)
		}
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
	RootCmd.Flags().IntVarP(&flagOffset, "offset", "o", 0, "Shift episode numbers (e.g. 12 to map Ep 1 to 13) (DB = Local + Offset)")
	RootCmd.Flags().StringVarP(&flagFillerURL, "filler", "F", "", "Override filler source URL")
	RootCmd.Flags().BoolVarP(&flagForce, "force", "f", false, "Force database refresh")
	RootCmd.Flags().BoolVarP(&flagNoTag, "no-tag", "T", false, "Disable MKV metadata tagging (mkvpropedit)")
	RootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress output except errors")

	// Default logger setup (before flags parse)
	l := log.New(os.Stdout)
	ui.SetLogger(l)
	ui.ConfigureLoggerStyles()
	logger = &ui.Logger{Logger: l}

	autotitle.SetDefaultEventHandler(func(e autotitle.Event) {
		msg := ui.ColorizeEvent(e.Message)
		switch e.Type {
		case autotitle.EventSuccess:
			logger.Success(msg)
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
		if _, ok := err.(types.ErrConfigNotFound); ok {
			logger.Error(fmt.Sprintf("No %s found in %s", ui.StylePattern.Render("_autotitle.yml"), ui.StylePath.Render(path)))
			fmt.Println()
			confirmInit := true
			err := ui.RunForm(huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Initialize now?").
						Description("Start the setup wizard to create a new configuration.").
						Value(&confirmInit),
				),
			).WithTheme(ui.AutotitleTheme()).WithKeyMap(ui.AutotitleKeyMap()))

			if err == nil && confirmInit {
				runInit(cmd, path)
				return
			}
			os.Exit(0)
		}
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
			ui.StyleCommand.Render(fmt.Sprint(success)),
			ui.StylePattern.Render(fmt.Sprint(skipped)),
			ui.StyleFlag.Render(fmt.Sprint(failed)),
		))
	}
}
