package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mydehq/autotitle/internal/matcher"
	"github.com/mydehq/autotitle/internal/provider"
)

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
		).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))
		if err != nil {
			return nil, err
		}
		return parseCommaSeparated(input), nil

	case 1:
		for {
			ClearAndPrintBanner(false)
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
			).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))
			if err != nil {
				return nil, err
			}

			if choice == "__custom__" {
				custom, err := promptCustomPatterns(theme)
				if err != nil {
					// Route the sentinel back to loop start
					if errors.Is(HandleAbort(err), ErrUserBack) {
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
			ClearAndPrintBanner(false)
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
			).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))
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
					if errors.Is(HandleAbort(err), ErrUserBack) {
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

	for {
		ClearAndPrintBanner(false)
		choice := ""
		err := RunForm(huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Output format\n").
					Options(opts...).
					Value(&choice),
			),
		).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))
		if err != nil {
			return nil, err
		}

		if choice == "__custom__" {
			input := ""
			for {
				ClearAndPrintBanner(false)
				err := RunForm(huh.NewForm(
					huh.NewGroup(
						huh.NewNote().
							Title("Output Format Legend").
							Description("\n• SERIES  — Series name (English)\n• EP\\_NUM  — Episode number (e.g. 01)\n• EP\\_NAME — Episode title\n• FILLER  — Filler tag (if detected)\n• RES     — Resolution (e.g. 1080p)\n• +       — Dynamic spacing/glue"),
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
				).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))
				if err != nil {
					if errors.Is(HandleAbort(err), ErrUserBack) {
						break
					}
					return nil, err
				}

				if strings.TrimSpace(input) == "" {
					break // break inner loop, go back to preset menu
				}

				parsed := parseCommaSeparated(input)

				preview := buildFilenamePreview(parsed, " ")
				confirm := true
				err = RunForm(huh.NewForm(
					huh.NewGroup(
						huh.NewNote().
							Title("Example output").
							Description(fmt.Sprintf("\nWith current settings, a file might be renamed to:\n\n  %s", preview)),
						huh.NewConfirm().
							Title("Use this format?").
							Value(&confirm),
					),
				).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))
				if err != nil {
					if errors.Is(HandleAbort(err), ErrUserBack) {
						continue // back to custom input
					}
					return nil, err
				}

				if confirm {
					return parsed, nil
				}
				// if !confirm, loop back, keep input string
			}
			continue
		}

		return strings.Split(choice, ","), nil
	}
}

func promptCustomPatterns(theme *huh.Theme) ([]string, error) {
	input := ""
	for {
		ClearAndPrintBanner(false)
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
		).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))
		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(input) == "" {
			return nil, ErrUserBack // explicitly use the sentinel for Back
		}

		parsed := parseCommaSeparated(input)

		var bulleted []string
		for _, p := range parsed {
			bulleted = append(bulleted, "  • "+p)
		}

		confirm := true
		err = RunForm(huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("Parsed patterns").
					Description(fmt.Sprintf("\n%s", strings.Join(bulleted, "\n"))),
				huh.NewConfirm().
					Title("Use these patterns?").
					Value(&confirm),
			),
		).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))
		if err != nil {
			if errors.Is(HandleAbort(err), ErrUserBack) {
				continue
			}
			return nil, err
		}

		if confirm {
			return parsed, nil
		}
	}
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
	).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(url), nil
}

// promptFillerURL prompts for the filler list URL, pre-filling a derived one.
func promptFillerURL(theme *huh.Theme, derived string) (string, error) {
	// Build legend dynamically from registered filler sources
	sources := provider.ListFillerSourceDetails()
	var lines []string
	for _, s := range sources {
		lines = append(lines, fmt.Sprintf("• %s - %s", s.Name, s.Website))
	}
	legend := strings.Join(lines, "\n")

	url := ""
	err := RunForm(huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Supported Filler Sites").
				Description("\n"+legend),
			huh.NewInput().
				Title("Filler URL").
				Description("\nIf this series has fillers, add filler list URL here.\nLeave empty to skip.\n").
				Placeholder(derived).
				Value(&url).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return nil // Optional
					}
					if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
						return fmt.Errorf("URL must start with http:// or https://")
					}
					return nil
				}),
		),
	).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(url), nil
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
