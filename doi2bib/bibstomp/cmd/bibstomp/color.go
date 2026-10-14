package main

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Row classes in the keys report. Orange marks a key that will be rewritten,
// teal one that survives as-is. AdaptiveColor is used rather than a fixed hex
// so the pair stays legible on both light and dark terminals: the dark-side
// values are brighter than the light-side ones by design.
var (
	changedStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#D75F00", Dark: "#FF8700",
	})
	unchangedStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#008080", Dark: "#00D7D7",
	})
)

// setColorMode configures the default renderer. In "auto" the profile that
// lipgloss detected for stdout is left alone, which means colour disappears
// the moment the report is piped into awk, cut or a file -- important, since
// this output is a TSV that other tools consume.
func setColorMode(mode string) error {
	r := lipgloss.DefaultRenderer()
	switch mode {
	case "never":
		r.SetColorProfile(termenv.Ascii)
	case "always":
		r.SetColorProfile(termenv.TrueColor)
	case "auto":
		// NO_COLOR is honoured explicitly: an empty-but-set value counts,
		// per the no-color.org convention.
		if _, ok := os.LookupEnv("NO_COLOR"); ok {
			r.SetColorProfile(termenv.Ascii)
		}
	default:
		return errBadColor
	}
	return nil
}

// renderRow formats one old<TAB>new pair, tinted by class.
func renderRow(old, new string, changed bool) string {
	line := old + "\t" + new
	if changed {
		return changedStyle.Render(line)
	}
	return unchangedStyle.Render(line)
}
