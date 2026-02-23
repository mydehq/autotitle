package cli

import (
	"regexp"

	"github.com/mydehq/autotitle/internal/ui"
	"github.com/spf13/cobra"
)

const coloredUsageTmpl = `{{Header "Usage:"}}
  {{if .Runnable}}{{Usage .UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{Command .CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

{{Header "Aliases:"}}
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

{{Header "Examples:"}}
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

{{Header "Available Commands:"}}{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{Command (printf "%-15s" .Name)}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{Header "Flags:"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces | Flags}}{{end}}{{if .HasAvailableInheritedFlags}}

{{Header "Global Flags:"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces | Flags}}{{end}}{{if .HasHelpSubCommands}}

{{Header "Additional help topics:"}}{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{Command (printf "%-15s" .Name)}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

{{Header "Use"}} {{Command (printf "%s [command] --help" .CommandPath)}} {{Header "for more information about a command."}}{{end}}
`

func colorizeHelp(cmd *cobra.Command) {
	cobra.AddTemplateFunc("Header", func(s string) string {
		out := ui.StyleHeader.Render(s)
		if s == "Usage:" {
			return "\n" + out
		}
		return out
	})
	cobra.AddTemplateFunc("Command", func(s string) string { return ui.StyleCommand.Render(s) })

	// Flags function colorizes individual flag names and dimmed separators
	cobra.AddTemplateFunc("Flags", func(s string) string {
		reFlags := regexp.MustCompile(`(-\w|--[\w-]+)`)
		s = reFlags.ReplaceAllStringFunc(s, func(match string) string {
			return ui.StyleFlag.Render(match)
		})

		reSep := regexp.MustCompile(`, `)
		s = reSep.ReplaceAllString(s, ui.StyleDim.Render(", "))

		return s
	})

	// Usage function colorizes the top-level usage line including args
	cobra.AddTemplateFunc("Usage", func(s string) string {
		// Colorize <args> (required) - Blue (StylePath)
		reArgs := regexp.MustCompile(`<[a-zA-Z0-9_-]+>`)
		s = reArgs.ReplaceAllStringFunc(s, func(match string) string {
			return ui.StylePath.Render(match)
		})

		// Colorize [args] (optional/flags) - Bright Dimmed (StyleDim)
		reOptional := regexp.MustCompile(`\[[a-zA-Z0-9_-]+\]`)
		s = reOptional.ReplaceAllStringFunc(s, func(match string) string {
			return ui.StyleDim.Render(match)
		})

		// Colorize the command name (beginning of the line) - Cyan (StyleCommand)
		reCmd := regexp.MustCompile(`^\w+`)
		s = reCmd.ReplaceAllStringFunc(s, func(match string) string {
			return ui.StyleCommand.Render(match)
		})

		return s
	})

	cmd.SetUsageTemplate(coloredUsageTmpl)
}
