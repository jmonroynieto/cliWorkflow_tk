// Package doi2bib retrieves DOI metadata and applies the personal BibTeX formatters.
package doi2bib

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/jmonroynieto/cliWorkflow_tk/doi2bib/bibstomp"
)

var doiPattern = regexp.MustCompile(`^10\.[0-9]{4,}/\S+$`)

func Normalize(input string) (string, error) {
	s := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, input)
	s = strings.NewReplacer("°", "/", "\"", "", "'", "").Replace(s)
	if i := strings.Index(s, "10."); i >= 0 {
		s = s[i:]
	}
	if !doiPattern.MatchString(s) {
		return "", fmt.Errorf("invalid DOI: %s", s)
	}
	return s, nil
}

type Formatter func(context.Context, string) (string, error)
type Client struct {
	HTTP *http.Client
	// BaseURL is optional, primarily for local testing.
	BaseURL string
	Format  Formatter
	// OnWarning reports optional formatters missing from PATH.
	OnWarning func(string)
}

func (c Client) Lookup(ctx context.Context, input string) (string, error) {
	doi, err := Normalize(input)
	if err != nil {
		return "", err
	}
	base := c.BaseURL
	if base == "" {
		base = "https://doi.org"
	}
	u, err := url.Parse(strings.TrimRight(base, "/") + "/")
	if err != nil {
		return "", err
	}
	u.Path += doi
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/x-bibtex")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("DOI lookup: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("DOI lookup: HTTP %d", resp.StatusCode)
	}
	if len(b) > 2*1024*1024 {
		return "", fmt.Errorf("DOI response too large")
	}
	raw := strings.TrimSpace(string(b))
	if !strings.HasPrefix(raw, "@") {
		return "", fmt.Errorf("DOI service did not return BibTeX")
	}
	format := c.Format
	if format == nil {
		format = func(ctx context.Context, raw string) (string, error) { return formatAvailable(ctx, raw, c.OnWarning) }
	}
	return format(ctx, raw)
}

// FormatExternal requires both formatters, retaining the strict pipeline API.
func FormatExternal(ctx context.Context, raw string) (string, error) {
	return formatPipeline(ctx, raw, false, nil)
}

func formatAvailable(ctx context.Context, raw string, warning func(string)) (string, error) {
	return formatPipeline(ctx, raw, true, warning)
}

func formatPipeline(ctx context.Context, raw string, optional bool, warning func(string)) (string, error) {
	for _, argv := range [][]string{{"bibtex-tidy"}, {"bibstomp", "stomp"}} {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		// Default lookups remint keys through the bibstomp Go library. The
		// strict FormatExternal API deliberately retains its PATH-only contract.
		if optional && argv[0] == "bibstomp" {
			out, err := bibstomp.Stomp(ctx, raw)
			if err != nil {
				return "", err
			}
			raw = out
			continue
		}
		if _, err := exec.LookPath(argv[0]); optional && errors.Is(err, exec.ErrNotFound) {
			if warning != nil {
				warning(argv[0] + " not found in PATH; skipped formatting step.")
			}
			continue
		}
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Stdin = strings.NewReader(raw)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("%s: %w: %s", argv[0], err, strings.TrimSpace(stderr.String()))
		}
		raw = string(out)
	}
	if !strings.HasPrefix(strings.TrimSpace(raw), "@") {
		return "", fmt.Errorf("formatters returned no BibTeX")
	}
	return strings.TrimSpace(raw), nil
}

func Fenced(bib string) string { return "```bibtex\n" + strings.TrimSpace(bib) + "\n```\n" }
