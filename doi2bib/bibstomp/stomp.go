package bibstomp

import (
	"context"
	"fmt"
	"strings"
)

// Stomp applies the CLI's default citation-key policy without invoking a
// subprocess or changing fields. Parser warnings are returned as errors.
// Missing authors/years use the
// same Anon/0000 defaults as the CLI; keys are not made artificially unique.
func Stomp(ctx context.Context, bib string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	src := []byte(bib)
	entries, warnings := ParseBib(src)
	if len(warnings) > 0 {
		return "", fmt.Errorf("bibstomp: %s", strings.Join(warnings, "; "))
	}
	var splices []Splice
	count := 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if entry.Special {
			continue
		}
		count++
		key, _ := MintKey(entry, DefaultKeyOpts())
		if key != entry.Key {
			splices = append(splices, Splice{entry.KeyStart, entry.KeyEnd, key})
		}
	}
	if count == 0 {
		return "", fmt.Errorf("bibstomp: no reference entries")
	}
	return string(ApplySplices(src, splices)), nil
}
