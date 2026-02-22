package cli

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
	"github.com/mydehq/autotitle/internal/matcher"
	"github.com/mydehq/autotitle/internal/provider/filler"
	"github.com/mydehq/autotitle/internal/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// handleAbort checks for user abort and exits cleanly.
// It maps huh.ErrUserAborted to ErrUserBack to implement our state machine navigation.
func handleAbort(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		if interceptedKey == "ctrl+c" {
			fmt.Println()
			logger.Info(StyleDim.Render("Init cancelled"))
			os.Exit(0)
		}
		return ErrUserBack
	}
	return err
}

// runInitWizard orchestrates the full interactive init wizard.
// search → select → patterns → preview → confirm.
func runInitWizard(ctx context.Context, cmd *cobra.Command, absPath string, scan *config.ScanResult) error {
	theme := autotitleTheme()

	// Wizard State
	step := 0

	searchQuery := filepath.Base(absPath)
	var selectedURL string
	var inputPatterns []string
	var outputFields []string
	defer autotitle.ClearSearchCache()
	autotitle.ClearSearchCache()

	for {
		ClearAndPrintBanner()
		switch step {
		case 0:
			// Editable search query
			err := RunForm(huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Search query").
						Description("\nEdit the query to search for your series").
						Value(&searchQuery),
				),
			).WithTheme(theme).WithKeyMap(autotitleKeyMap()))

			if err != nil {
				if errors.Is(handleAbort(err), ErrUserBack) {
					// We are at the first step, so "back" means abort.
					fmt.Println()
					logger.Info(StyleDim.Render("Init cancelled"))
					os.Exit(0)
				}
				return err
			}
			step++

		case 1:
			// Live streaming search across all providers
			url, err := runStreamingSearch(ctx, searchQuery)
			if err != nil {
				if errors.Is(handleAbort(err), ErrUserBack) {
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
					if errors.Is(handleAbort(manualErr), ErrUserBack) {
						continue
					}
					return manualErr
				}
			} else {
				selectedURL = url
			}
			step++

		case 2:
			// Pattern selection
			var err error
			inputPatterns, err = selectInputPatterns(scan.DetectedPatterns, theme)
			if err != nil {
				if errors.Is(handleAbort(err), ErrUserBack) {
					step--
					continue
				}
				return err
			}
			step++

		case 3:
			// Output fields
			var err error
			outputFields, err = selectOutputFields(theme)
			if err != nil {
				if errors.Is(handleAbort(err), ErrUserBack) {
					step--
					continue
				}
				return err
			}
			step++

		case 4:
			// Live filename preview
			preview := buildFilenamePreview(outputFields, " ")
			err := RunForm(huh.NewForm(
				huh.NewGroup(
					huh.NewNote().
						Title("Example output").
						Description(fmt.Sprintf("\nWith current settings, a file might be renamed to:\n\n  %s", preview)),
				),
			).WithTheme(theme).WithKeyMap(autotitleKeyMap()))

			if err != nil {
				if errors.Is(handleAbort(err), ErrUserBack) {
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
			fillerURL := filler.DeriveURLFromProvider(selectedURL)

			if cmd != nil && cmd.Flags().Changed("filler") {
				fillerURL = flagInitFillerURL
			}
			if cmd != nil && cmd.Flags().Changed("separator") {
				separator = flagInitSeparator
			}
			if cmd != nil && cmd.Flags().Changed("offset") {
				offsetStr = strconv.Itoa(flagInitOffset)
			}
			if cmd != nil && cmd.Flags().Changed("padding") {
				paddingStr = strconv.Itoa(flagInitPadding)
			}

			var refinementFields []huh.Field
			if cmd == nil || !cmd.Flags().Changed("filler") {
				refinementFields = append(refinementFields,
					huh.NewInput().
						Title("Filler URL").
						Description("\nOptional. Clear to skip.").
						Value(&fillerURL),
				)
			}
			if cmd == nil || !cmd.Flags().Changed("separator") {
				refinementFields = append(refinementFields,
					huh.NewInput().
						Title("Separator").
						Description("\nCharacter(s) between output fields").
						Value(&separator),
				)
			}
			if cmd == nil || !cmd.Flags().Changed("offset") {
				refinementFields = append(refinementFields,
					huh.NewInput().
						Title("Episode offset").
						Description("\nOptional. Maps local → DB episode numbers").
						Value(&offsetStr).
						Validate(validateInt),
				)
			}
			if cmd == nil || !cmd.Flags().Changed("padding") {
				refinementFields = append(refinementFields,
					huh.NewInput().
						Title("Episode padding").
						Description("\nOptional. Force digit width (e.g. 2 → 01)").
						Value(&paddingStr).
						Validate(validateInt),
				)
			}

			if len(refinementFields) > 0 {
				err := RunForm(huh.NewForm(
					huh.NewGroup(refinementFields...),
				).WithTheme(theme).WithKeyMap(autotitleKeyMap()))
				if err != nil {
					if errors.Is(handleAbort(err), ErrUserBack) {
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
				if errors.Is(handleAbort(err), ErrUserBack) {
					step-- // go back
					continue
				}
				return err
			}
			if !confirmed {
				fmt.Println()
				logger.Info(StyleDim.Render("Init cancelled"))
				os.Exit(0)
				return nil
			}

			// Save config
			if err := config.SaveToDir(absPath, cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}
			mapPath := filepath.Join(absPath, config.GetDefaults().MapFile)
			logger.Info(fmt.Sprintf("%s: %s", StyleHeader.Render("Created config"), StylePath.Render(mapPath)))

			// ─Offer DB generation
			if flagDryRun {
				logger.Info(StyleDim.Render("[DRY RUN] Skipping DB generation prompt"))
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
			).WithTheme(theme).WithKeyMap(autotitleKeyMap()))

			// If user presses BACK here, theoretically they can't undo the config save,
			// so we just cancel the db fetch.
			if err != nil && !errors.Is(handleAbort(err), ErrUserBack) {
				return handleAbort(err) // propagate real errors
			}

			if fetchDB {
				opts := []autotitle.Option{}
				if fillerURL != "" {
					opts = append(opts, autotitle.WithFiller(fillerURL))
				}
				_, err := autotitle.DBGen(ctx, selectedURL, opts...)
				if err != nil {
					logger.Error("Failed to generate database", "error", err)
				} else {
					logger.Info(fmt.Sprintf("%s: %s", StyleHeader.Render("Database generated"), StylePath.Render(selectedURL)))
				}
			}

			return nil
		}
	}
}

// selectInputPatterns implements the pattern selection step with adaptive widgets.
func selectInputPatterns(detected []string, theme *huh.Theme) ([]string, error) {
	switch len(detected) {
	case 0:
		// No patterns detected: free-form input
		input := ""
		err := RunForm(huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Input patterns").
					Description("\nEnter patterns (comma-separated). Placeholders: {{EP_NUM}}, {{SERIES}}, {{RES}}, {{ANY}}, {{EXT}}").
					Value(&input).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("at least one pattern is required")
						}
						for _, p := range strings.Split(s, ",") {
							p = strings.TrimSpace(p)
							if p == "" {
								continue
							}
							if _, err := matcher.Compile(p); err != nil {
								return fmt.Errorf("invalid pattern %q: %w", p, err)
							}
						}
						return nil
					}),
			),
		).WithTheme(theme).WithKeyMap(autotitleKeyMap()))
		if err != nil {
			return nil, err
		}
		return parseCommaSeparated(input), nil

	case 1:
		for {
			ClearAndPrintBanner()
			// One pattern: select it or add custom
			choice := ""
			err := RunForm(huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Input pattern detected\n").
						Options(
							huh.NewOption(detected[0], detected[0]),
							huh.NewOption("Add custom pattern...", "__custom__"),
						).
						Value(&choice),
				),
			).WithTheme(theme).WithKeyMap(autotitleKeyMap()))
			if err != nil {
				return nil, err
			}

			if choice == "__custom__" {
				custom, err := promptCustomPatterns(theme)
				if err != nil {
					// Route the sentinel back to loop start
					if errors.Is(handleAbort(err), ErrUserBack) {
						continue
					}
					return nil, err
				}
				if len(custom) == 0 {
					continue // Empty string back navigation
				}
				return custom, nil
			}
			return []string{choice}, nil
		}

	default:
		for {
			ClearAndPrintBanner()
			// Multiple patterns: multi-select with all pre-checked
			allChoices := make([]string, len(detected))
			copy(allChoices, detected)

			selected := make([]string, len(detected))
			copy(selected, detected)

			multiOpts := make([]huh.Option[string], 0, len(detected)+1)
			for _, d := range detected {
				multiOpts = append(multiOpts, huh.NewOption(d, d).Selected(true))
			}
			multiOpts = append(multiOpts, huh.NewOption("Add custom pattern...", "__custom__"))

			var err error
			err = RunForm(huh.NewForm(
				huh.NewGroup(
					huh.NewMultiSelect[string]().
						Title("Input patterns detected\n").
						Description("Uncheck patterns you don't want\n").
						Options(multiOpts...).
						Value(&selected),
				),
			).WithTheme(theme).WithKeyMap(autotitleKeyMap()))
			if err != nil {
				return nil, err
			}

			// Check if custom was selected
			hasCustom := false
			var finalPatterns []string
			for _, s := range selected {
				if s == "__custom__" {
					hasCustom = true
				} else {
					finalPatterns = append(finalPatterns, s)
				}
			}

			if hasCustom {
				custom, err := promptCustomPatterns(theme)
				if err != nil {
					// Route the sentinel/err back to loop start
					if errors.Is(handleAbort(err), ErrUserBack) {
						continue
					}
					return nil, err
				}
				if len(custom) == 0 {
					continue // Empty string back navigation
				}
				finalPatterns = append(finalPatterns, custom...)
			}

			return finalPatterns, nil
		}
	}
}

// selectOutputFields implements the output field preset selection step.
func selectOutputFields(theme *huh.Theme) ([]string, error) {
	type preset struct {
		name   string
		fields []string
	}
	presets := []preset{
		{"Default", []string{"E", "+", "EP_NUM", "FILLER", "-", "EP_NAME"}},
		{"Minimal", []string{"EP_NUM", "-", "EP_NAME"}},
		{"Full", []string{"SERIES", "-", "EP_NUM", "-", "EP_NAME"}},
		{"Custom", nil},
	}

	opts := make([]huh.Option[string], len(presets))
	for i, p := range presets {
		val := strings.Join(p.fields, ",")
		label := p.name
		if p.fields != nil {
			preview := buildFilenamePreview(p.fields, " ")
			label = fmt.Sprintf("%-8s (%s)", p.name, preview)
		} else {
			val = "__custom__"
		}
		opts[i] = huh.NewOption(label, val)
	}

	choice := ""
	err := RunForm(huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Output Format Legend").
				Description("\n• SERIES — Series name (English)\n• EP\\_NUM — Episode number (e.g. 01)\n• EP\\_NAME — Episode title\n• FILLER — Filler tag (if detected)\n• RES    — Resolution (e.g. 1080p)\n• +      — Dynamic spacing/glue"),

			huh.NewSelect[string]().
				Title("Output format\n").
				Options(opts...).
				Value(&choice),
		),
	).WithTheme(theme).WithKeyMap(autotitleKeyMap()))
	if err != nil {
		return nil, err
	}

	if choice == "__custom__" {
		input := ""
		err := RunForm(huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Custom output fields").
					Description("\nEnter fields (comma-separated). e.g: SERIES, -, EP_NUM, -, EP_NAME").
					Value(&input).
					Validate(func(s string) error {
						// we allow empty for going back!
						if strings.TrimSpace(s) == "" {
							return nil
						}
						// Let's assume valid
						return nil
					}),
			),
		).WithTheme(theme).WithKeyMap(autotitleKeyMap()))
		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(input) == "" {
			return nil, ErrUserBack
		}

		return parseCommaSeparated(input), nil
	}

	return strings.Split(choice, ","), nil
}

// buildFilenamePreview creates an example filename using mock episode data.
func buildFilenamePreview(outputFields []string, separator string) string {
	vars := matcher.TemplateVars{
		Series:   "Bleach",
		SeriesEn: "Bleach",
		SeriesJp: "ブリーチ",
		EpNum:    "1",
		EpName:   "The Day I Became a Shinigami",
		Res:      "1080p",
		Ext:      "mkv",
	}

	if separator == "" {
		separator = " "
	}

	name, _ := matcher.GenerateFilenameFromFields(outputFields, separator, vars, 2)
	return name
}

// showPreviewAndConfirm marshals the config to YAML and shows a confirmation prompt.
func showPreviewAndConfirm(cfg *types.Config, theme *huh.Theme) (bool, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return false, fmt.Errorf("failed to preview config: %w", err)
	}

	confirmed := false
	err = RunForm(huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Config preview").
				Description(string(data)),
			huh.NewConfirm().
				Title("Write config?").
				Value(&confirmed),
		),
	).WithTheme(theme).WithKeyMap(autotitleKeyMap()))
	if err != nil {
		return false, err
	}

	return confirmed, nil
}

// promptManualURL opens a validated URL input.
func promptManualURL(theme *huh.Theme) (string, error) {
	url := ""
	err := RunForm(huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Provider URL").
				Description("\nEnter a MAL, TMDB, or other supported provider URL").
				Value(&url).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return fmt.Errorf("URL is required")
					}
					if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
						return fmt.Errorf("URL must start with http:// or https://")
					}
					return nil
				}),
		),
	).WithTheme(theme).WithKeyMap(autotitleKeyMap()))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(url), nil
}

// promptCustomPatterns asks for comma-separated patterns, allowing empty submission to 'go back'.
func promptCustomPatterns(theme *huh.Theme) ([]string, error) {
	input := ""
	err := RunForm(huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Input Placeholder Legend").
				Description("\n• {{SERIES}} — Matches series name\n• {{EP\\_NUM}} — Matches episode number\n• {{RES}}    — Matches resolution (e.g. 1080p)\n• {{ANY}}    — Matches any character(s)\n• {{EXT}}    — Matches file extension"),
			huh.NewInput().
				Title("Custom input patterns").
				Description("\nLeave empty to go back").
				Value(&input).
				Validate(func(s string) error {
					// empty is ok, we handle it as 'back'
					if strings.TrimSpace(s) == "" {
						return nil
					}
					for _, p := range strings.Split(s, ",") {
						p = strings.TrimSpace(p)
						if p == "" {
							continue
						}
						if _, err := matcher.Compile(p); err != nil {
							return fmt.Errorf("invalid pattern %q: %w", p, err)
						}
					}
					return nil
				}),
		),
	).WithTheme(theme).WithKeyMap(autotitleKeyMap()))
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(input) == "" {
		return nil, ErrUserBack // explicitly use the sentinel for Back
	}
	return parseCommaSeparated(input), nil
}

// parseCommaSeparated splits a comma-separated string into trimmed, non-empty parts.
func parseCommaSeparated(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// validateInt validates that a string can be parsed as an integer.
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
