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
// Returns true if the user wants to start renaming immediately.
func RunInitWizard(ctx context.Context, absPath string, scan *config.ScanResult, flags InitFlags) (bool, error) {
	theme := AutotitleTheme()

	// Wizard State
	step := 0

	searchQuery := filepath.Base(absPath)
	var selectedURL string
	var fillerURL string
	var inputPatterns []string
	var outputFields []string
	var showAdvanced bool

	separator := " "
	offsetStr := "0"
	paddingStr := "0"

	if flags.HasSeparator {
		separator = flags.Separator
	}
	if flags.HasOffset {
		offsetStr = strconv.Itoa(flags.Offset)
	}
	if flags.HasPadding {
		paddingStr = strconv.Itoa(flags.Padding)
	}

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
						logger.Warn(StyleDim.Render("Init cancelled"))
					}
					os.Exit(0)
				}
				return false, err
			}
			step++

		case 1:
			// Live streaming search across all providers
			url, err := runStreamingSearch(ctx, searchQuery) // Note: small 'r'
			if err != nil {
				if errors.Is(err, ErrSearchAgain) {
					step--
					continue
				}
				if errors.Is(HandleAbort(err), ErrUserBack) {
					step--
					continue
				}
				return false, err
			}
			if url == "" {
				// No results or user chose manual entry
				var manualErr error
				selectedURL, manualErr = promptManualURL(theme)
				if manualErr != nil {
					if errors.Is(HandleAbort(manualErr), ErrUserBack) {
						continue
					}
					return false, manualErr
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
				return false, err
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
				return false, err
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
				return false, err
			}
			step++

		case 5:
			// Episode offset
			if flags.HasOffset {
				step++
				continue
			}

			err := RunForm(huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Episode offset").
						Description("\nShift episode numbers (DB = Local + Offset).\nUse 12 to map Local E01 to Database E13.\n").
						Value(&offsetStr).
						Validate(validateInt),
				),
			).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))
			if err != nil {
				if errors.Is(HandleAbort(err), ErrUserBack) {
					step--
					continue
				}
				return false, err
			}
			step++

		case 6:
			// Ask for advanced settings
			if flags.HasSeparator && flags.HasPadding {
				step++
				continue
			}

			err := RunForm(huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Advanced Settings").
						Description("Do you want to configure additional settings (Separator, Episode Padding..)?").
						Value(&showAdvanced),
				),
			).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))
			if err != nil {
				if errors.Is(HandleAbort(err), ErrUserBack) {
					step--
					continue
				}
				return false, err
			}
			step++

		case 7:
			// Optional refinement fields
			if !showAdvanced {
				step++
				continue
			}

			var refinementFields []huh.Field
			if !flags.HasSeparator {
				refinementFields = append(refinementFields,
					huh.NewInput().
						Title("Separator").
						Placeholder(" ").
						Description("\nCharacter(s) between output fields").
						Value(&separator),
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
					return false, err
				}
			}
			step++

		case 8:
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
				return false, err
			}
			if !confirmed {
				fmt.Println()
				if logger != nil {
					logger.Warn(StyleDim.Render("Init cancelled"))
				}
				os.Exit(0)
				return false, nil
			}

			// Save config
			if err := config.SaveToDir(absPath, cfg); err != nil {
				return false, fmt.Errorf("failed to save config: %w", err)
			}
			step++

		case 9:
			// Final success and ask to start renaming
			mapPath := filepath.Join(absPath, config.GetDefaults().MapFile)

			if logger != nil {
				logger.Success(fmt.Sprintf("%s %s", StyleHeader.Render("Configuration saved to:"), StylePath.Render(mapPath)))
				fmt.Println()
			}

			// Offer renaming
			startRename := true
			err := RunForm(huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Start renaming now?").
						Value(&startRename),
				),
			).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))

			if err != nil {
				return false, HandleAbort(err) // No going back from the final success screen
			}

			return startRename, nil
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
				logger.Warn(StyleDim.Render("Init cancelled"))
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
