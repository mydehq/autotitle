package ui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/mydehq/autotitle/internal/matcher"
	"github.com/mydehq/autotitle/internal/types"
	"gopkg.in/yaml.v3"
)

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

	confirmed := true
	err = RunForm(huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Configuration Preview").
				Description(fmt.Sprintf("\n%s\n\n", HighlightYAML(string(data)))),

			huh.NewConfirm().
				Title("Write configuration?").
				Value(&confirmed),
		),
	).WithTheme(theme).WithKeyMap(AutotitleKeyMap()))
	if err != nil {
		return false, err
	}

	return confirmed, nil
}
