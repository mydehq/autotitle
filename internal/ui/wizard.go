package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mydehq/autotitle"
	"github.com/mydehq/autotitle/internal/config"
	"github.com/mydehq/autotitle/internal/provider/filler"
)

// InitFlags encapsulates all the CLI flags required by the init wizard.
type InitFlags struct {
	URL          string
	FillerURL    string
	HasFiller    bool
	Separator    string
	HasSeparator bool
	Offset       int
	HasOffset    bool
	Padding      int
	HasPadding   bool
	DryRun       bool
}

// RunInitWizard orchestrates the full interactive init wizard.
// search → select → patterns → preview → confirm.
func RunInitWizard(ctx context.Context, absPath string, scan *config.ScanResult, flags InitFlags) error {
	theme := AutotitleTheme()

	// Wizard State
	step := 0

	searchQuery := filepath.Base(absPath)
	var selectedURL string
	var fillerURL string
	var inputPatterns []string
	var outputFields []string
	defer autotitle.ClearSearchCache()
	autotitle.ClearSearchCache()

	for {
		ClearAndPrintBanner(flags.DryRun)
		switch step {
		case 0:
			// Editable search query
			err := RunForm(huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Search query").
						Description("\nEdit the query to search for your series\n").
						Value(&searchQuery),
				),
			).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))

			if err != nil {
				if errors.Is(HandleAbort(err), ErrUserBack) {
					// We are at the first step, so "back" means abort.
					fmt.Println()
					if logger != nil {
						logger.Info(StyleDim.Render("Init cancelled"))
					}
					os.Exit(0)
				}
				return err
			}
			step++

		case 1:
			// Live streaming search across all providers
			url, err := runStreamingSearch(ctx, searchQuery) // Note: small 'r'
			if err != nil {
				if errors.Is(HandleAbort(err), ErrUserBack) {
					step--
					continue
				}
				return err
			}
			if url == "" {
				// No results or user chose manual entry
				var manualErr error
				selectedURL, manualErr = promptManualURL(theme)
				if manualErr != nil {
					if errors.Is(HandleAbort(manualErr), ErrUserBack) {
						continue
					}
					return manualErr
				}
			} else {
				selectedURL = url
			}
			step++

		case 2:
			// Filler URL selection
			if flags.HasFiller {
				fillerURL = flags.FillerURL
				step++
				continue
			}

			derived := filler.DeriveURLFromProvider(selectedURL)
			var err error
			fillerURL, err = promptFillerURL(theme, derived)
			if err != nil {
				if errors.Is(HandleAbort(err), ErrUserBack) {
					step--
					continue
				}
				return err
			}
			step++

		case 3:
			// Pattern selection
			var err error
			inputPatterns, err = selectInputPatterns(scan.DetectedPatterns, theme)
			if err != nil {
				if errors.Is(HandleAbort(err), ErrUserBack) {
					step--
					continue
				}
				return err
			}
			step++

		case 4:
			// Output fields
			var err error
			outputFields, err = selectOutputFields(theme)
			if err != nil {
				if errors.Is(HandleAbort(err), ErrUserBack) {
					step--
					continue
				}
				return err
			}
			step++

		case 5:
			// Optional refinement fields
			paddingStr := "0"
			offsetStr := "0"
			separator := " "

			if flags.HasSeparator {
				separator = flags.Separator
			}
			if flags.HasOffset {
				offsetStr = strconv.Itoa(flags.Offset)
			}
			if flags.HasPadding {
				paddingStr = strconv.Itoa(flags.Padding)
			}

			var refinementFields []huh.Field
			if !flags.HasSeparator {
				refinementFields = append(refinementFields,
					huh.NewInput().
						Title("Separator").
						Description("\nCharacter(s) between output fields").
						Value(&separator),
				)
			}
			if !flags.HasOffset {
				refinementFields = append(refinementFields,
					huh.NewInput().
						Title("Episode offset").
						Description("\nOptional. Maps local → DB episode numbers").
						Value(&offsetStr).
						Validate(validateInt),
				)
			}
			if !flags.HasPadding {
				refinementFields = append(refinementFields,
					huh.NewInput().
						Title("Episode padding").
						Description("\nOptional. Force digit width (e.g. 2 → E01)").
						Value(&paddingStr).
						Validate(validateInt),
				)
			}

			if len(refinementFields) > 0 {
				err := RunForm(huh.NewForm(
					huh.NewGroup(refinementFields...),
				).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))
				if err != nil {
					if errors.Is(HandleAbort(err), ErrUserBack) {
						step--
						continue
					}
					return err
				}
			}

			offset, _ := strconv.Atoi(offsetStr)
			padding, _ := strconv.Atoi(paddingStr)

			// Build config
			cfg := config.GenerateDefault(selectedURL, fillerURL, inputPatterns, separator, offset, padding)
			if len(cfg.Targets) > 0 && len(cfg.Targets[0].Patterns) > 0 {
				cfg.Targets[0].Patterns[0].Output.Fields = outputFields
			}

			// Preview YAML, confirm
			confirmed, err := showPreviewAndConfirm(cfg, theme)
			if err != nil {
				if errors.Is(HandleAbort(err), ErrUserBack) {
					step-- // go back
					continue
				}
				return err
			}
			if !confirmed {
				fmt.Println()
				if logger != nil {
					logger.Info(StyleDim.Render("Init cancelled"))
				}
				os.Exit(0)
				return nil
			}

			// Save config
			if err := config.SaveToDir(absPath, cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}
			mapPath := filepath.Join(absPath, config.GetDefaults().MapFile)
			if logger != nil {
				logger.Info(fmt.Sprintf("%s: %s", StyleHeader.Render("Created config"), StylePath.Render(mapPath)))
			}

			// ─Offer DB generation
			if flags.DryRun {
				if logger != nil {
					logger.Info(StyleDim.Render("[DRY RUN] Skipping DB generation prompt"))
				}
				return nil // done!
			}

			fetchDB := false
			err = RunForm(huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Fetch database now?").
						Description("\nDownload episode data from the provider").
						Value(&fetchDB),
				),
			).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))

			// If user presses BACK here, theoretically they can't undo the config save,
			// so we just cancel the db fetch.
			if err != nil && !errors.Is(HandleAbort(err), ErrUserBack) {
				return HandleAbort(err) // propagate real errors
			}

			if fetchDB {
				opts := []autotitle.Option{}
				if fillerURL != "" {
					opts = append(opts, autotitle.WithFiller(fillerURL))
				}
				_, err := autotitle.DBGen(ctx, selectedURL, opts...)
				if err != nil {
					if logger != nil {
						logger.Error("Failed to generate database", "error", err)
					}
				} else {
					if logger != nil {
						logger.Info(fmt.Sprintf("%s: %s", StyleHeader.Render("Database generated"), StylePath.Render(selectedURL)))
					}
				}
			}

			return nil
		}
	}
}

// handleAbort checks for user abort and exits cleanly.
// It maps huh.ErrUserAborted to ErrUserBack to implement our state machine navigation.
func HandleAbort(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		if interceptedKey == "ctrl+c" {
			fmt.Println()
			if logger != nil {
				logger.Info(StyleDim.Render("Init cancelled"))
			}
			os.Exit(0)
		}
		return ErrUserBack
	}
	return err
}

func validateInt(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if _, err := strconv.Atoi(s); err != nil {
		return fmt.Errorf("must be a number")
	}
	return nil
}
