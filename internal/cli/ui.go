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
	"github.com/charmbracelet/huh/spinner"
	"github.com/mydehq/autotitle"
	"github.com/mydehq/autotitle/internal/config"
	"github.com/mydehq/autotitle/internal/matcher"
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

	for {
		ClearAndPrintBanner()
		switch step {
		case 0:
			// Editable search query
			err := RunForm(huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Search query").
						Description("Edit the query to search for your series").
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
			// Search all providers
			for {
				results, searchErr := searchWithSpinner(ctx, searchQuery)

				if searchErr != nil {
					// Show error and offer retry/manual
					choice := ""
					err := RunForm(huh.NewForm(
						huh.NewGroup(
							huh.NewNote().
								Title("Search failed").
								Description(fmt.Sprintf("Error: %s", searchErr.Error())),
							huh.NewSelect[string]().
								Title("What would you like to do?").
								Options(
									huh.NewOption("Search again", "retry"),
									huh.NewOption("Enter URL manually", "manual"),
									huh.NewOption("Go back", "back"),
								).
								Value(&choice),
						),
					).WithTheme(theme).WithKeyMap(autotitleKeyMap()))
					if err != nil {
						if errors.Is(handleAbort(err), ErrUserBack) {
							step--
							break
						}
						return err
					}

					if choice == "back" {
						step--
						break
					}
					if choice == "manual" {
						var err error
						selectedURL, err = promptManualURL(theme)
						if err != nil {
							if errors.Is(handleAbort(err), ErrUserBack) {
								continue
							}
							return err
						}
						step++
						break
					}

					// retry: edit query and loop
					err = RunForm(huh.NewForm(
						huh.NewGroup(
							huh.NewInput().
								Title("Search query").
								Value(&searchQuery),
						),
					).WithTheme(theme).WithKeyMap(autotitleKeyMap()))
					if err != nil {
						if errors.Is(handleAbort(err), ErrUserBack) {
							continue
						}
						return err
					}
					continue
				}

				if len(results) == 0 {
					choice := ""
					err := RunForm(huh.NewForm(
						huh.NewGroup(
							huh.NewNote().
								Title("No results").
								Description(fmt.Sprintf("No results found for %q", searchQuery)),
							huh.NewSelect[string]().
								Title("What would you like to do?").
								Options(
									huh.NewOption("Search again", "retry"),
									huh.NewOption("Enter URL manually", "manual"),
									huh.NewOption("Go back", "back"),
								).
								Value(&choice),
						),
					).WithTheme(theme).WithKeyMap(autotitleKeyMap()))
					if err != nil {
						if errors.Is(handleAbort(err), ErrUserBack) {
							step--
							break
						}
						return err
					}

					if choice == "back" {
						step--
						break
					}
					if choice == "manual" {
						var err error
						selectedURL, err = promptManualURL(theme)
						if err != nil {
							if errors.Is(handleAbort(err), ErrUserBack) {
								continue
							}
							return err
						}
						step++
						break
					}
					// retry: edit query
					err = RunForm(huh.NewForm(
						huh.NewGroup(
							huh.NewInput().
								Title("Search query").
								Value(&searchQuery),
						),
					).WithTheme(theme).WithKeyMap(autotitleKeyMap()))
					if err != nil {
						if errors.Is(handleAbort(err), ErrUserBack) {
							continue
						}
						return err
					}
					continue
				}

				// Show results for selection
				opts := buildProviderOptions(results)
				sel := huh.NewSelect[string]().
					Title("Select your series").
					Description(fmt.Sprintf("Found %d results", len(results))).
					Options(opts...).
					Value(&selectedURL).
					Filtering(true)
				if len(results) > 10 {
					sel = sel.Height(15)
				}
				err := RunForm(huh.NewForm(
					huh.NewGroup(sel),
				).WithTheme(theme).WithKeyMap(autotitleKeyMap()))
				if err != nil {
					if errors.Is(handleAbort(err), ErrUserBack) {
						step--
						break
					}
					return err
				}

				step++
				break
			}

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
			preview := buildFilenamePreview(inputPatterns, outputFields, " ")
			err := RunForm(huh.NewForm(
				huh.NewGroup(
					huh.NewNote().
						Title("Example output").
						Description(fmt.Sprintf("With current settings, a file might be renamed to:\n\n  %s", preview)),
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
			fillerURL := deriveFillerURL(selectedURL)

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
						Description("Optional. Clear to skip.").
						Value(&fillerURL),
				)
			}
			if cmd == nil || !cmd.Flags().Changed("separator") {
				refinementFields = append(refinementFields,
					huh.NewInput().
						Title("Separator").
						Description("Character(s) between output fields").
						Value(&separator),
				)
			}
			if cmd == nil || !cmd.Flags().Changed("offset") {
				refinementFields = append(refinementFields,
					huh.NewInput().
						Title("Episode offset").
						Description("Optional. Maps local → DB episode numbers").
						Value(&offsetStr).
						Validate(validateInt),
				)
			}
			if cmd == nil || !cmd.Flags().Changed("padding") {
				refinementFields = append(refinementFields,
					huh.NewInput().
						Title("Episode padding").
						Description("Optional. Force digit width (e.g. 2 → 01)").
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
			defaults := config.GetDefaults()
			mapFileName := defaults.MapFile
			mapPath := filepath.Join(absPath, mapFileName)

			if err := config.Save(mapPath, cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}
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
						Description("Download episode data from the provider").
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

// searchWithSpinner runs autotitle.Search across all providers with a spinner.
func searchWithSpinner(ctx context.Context, query string) ([]types.SearchResult, error) {
	var results []types.SearchResult
	var searchErr error

	err := spinner.New().
		Title(fmt.Sprintf("%s %s", StyleDim.Render("Searching for"), StyleCommand.Render(query))).
		Action(func() {
			results, searchErr = autotitle.Search(ctx, query)
		}).
		Run()

	if err != nil {
		return nil, fmt.Errorf("spinner failed: %w", err)
	}
	if searchErr != nil {
		return nil, searchErr
	}

	fmt.Printf("  %s %s\n\n", StyleDim.Render("Found"), StyleCommand.Render(fmt.Sprintf("%d results", len(results))))
	return results, nil
}

// buildProviderOptions maps search results to huh.Option with formatted labels.
func buildProviderOptions(results []types.SearchResult) []huh.Option[string] {
	opts := make([]huh.Option[string], len(results))
	for i, r := range results {
		label := r.Title
		if r.Year > 0 {
			label += fmt.Sprintf(" (%d)", r.Year)
		}
		label += " " + StyleDim.Render("["+strings.ToUpper(r.Provider)+"]")
		opts[i] = huh.NewOption(label, r.URL)
	}
	return opts
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
					Description("Enter patterns (comma-separated). Placeholders: {{EP_NUM}}, {{SERIES}}, {{RES}}, {{ANY}}, {{EXT}}").
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
			// One pattern: select it or add custom
			choice := ""
			err := RunForm(huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Input pattern detected").
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
						Title("Input patterns detected").
						Description("Uncheck patterns you don't want").
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
		{"Default  (E + EP_NUM FILLER - EP_NAME)", []string{"E", "+", "EP_NUM", "FILLER", "-", "EP_NAME"}},
		{"Minimal  (EP_NUM - EP_NAME)", []string{"EP_NUM", "-", "EP_NAME"}},
		{"Full     (SERIES - EP_NUM - EP_NAME)", []string{"SERIES", "-", "EP_NUM", "-", "EP_NAME"}},
		{"Custom", nil},
	}

	opts := make([]huh.Option[string], len(presets))
	for i, p := range presets {
		val := strings.Join(p.fields, ",")
		if p.fields == nil {
			val = "__custom__"
		}
		opts[i] = huh.NewOption(p.name, val)
	}

	choice := ""
	err := RunForm(huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Output format").
				Description("Tokens: SERIES, EP_NUM, EP_NAME, FILLER, RES, + (glue), literals").
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
					Description("Enter fields (comma-separated). e.g: SERIES, -, EP_NUM, -, EP_NAME").
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
func buildFilenamePreview(inputPatterns, outputFields []string, separator string) string {
	// Build from output fields with mock data
	fieldMap := map[string]string{
		"SERIES":    "Bleach",
		"SERIES_EN": "Bleach",
		"SERIES_JP": "ブリーチ",
		"EP_NUM":    "01",
		"EP_NAME":   "The Day I Became a Shinigami",
		"FILLER":    "",
		"RES":       "1080p",
		"E":         "E",
		"+":         "",
		"-":         "-",
	}

	var parts []string
	for _, f := range outputFields {
		if val, ok := fieldMap[f]; ok {
			if val != "" {
				parts = append(parts, val)
			}
		} else {
			// Literal
			parts = append(parts, f)
		}
	}

	if separator == "" {
		separator = " "
	}

	return strings.Join(parts, separator) + ".mkv"
}

// deriveFillerURL derives an AnimeFillerList URL from the selected provider URL.
func deriveFillerURL(providerURL string) string {
	// Extract a series name from the URL and convert to filler-list slug
	// e.g. https://myanimelist.net/anime/269/Bleach → bleach
	parts := strings.Split(providerURL, "/")
	if len(parts) > 0 {
		slug := parts[len(parts)-1]
		slug = strings.ToLower(slug)
		slug = strings.ReplaceAll(slug, "_", "-")
		slug = strings.ReplaceAll(slug, " ", "-")
		if slug != "" {
			return fmt.Sprintf("https://www.animefillerlist.com/shows/%s", slug)
		}
	}
	return ""
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
				Description("Enter a MAL, TMDB, or other supported provider URL").
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
			huh.NewInput().
				Title("Custom input patterns").
				Description("Leave empty to go back. Placeholders: {{EP_NUM}}, {{SERIES}}, {{RES}}, {{ANY}}, {{EXT}}").
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
