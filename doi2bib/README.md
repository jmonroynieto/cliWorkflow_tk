# doi2bib

`doi2bib` is a cliWorkflow_tk module with a library at its root and a CLI in
`cmd/doi2bib`. It retrieves DOI metadata as BibTeX, then uses the in-tree
`bibstomp` package to remint citation keys.

From the toolkit root, build and install it with the usual commands:

```sh
make build
make install
```

For focused development:
```sh
go build -o doi2bib ./cmd/doi2bib
go test ./...
./doi2bib                         # prompt until an empty line or EOF
./doi2bib --oneshot dois.txt       # one DOI per line, including a final line without newline
./doi2bib --oneshot dois.txt --bare # omit Markdown fences
./doi2bib --no-clipboard           # useful outside X11
```

Formatting runs `bibtex-tidy` when available, then calls `bibstomp.Stomp` directly in Go to remint citation keys. The bibstomp executable, PATH lookup, extraction, and Linux/glibc restriction are no longer part of the default pipeline.

A missing `bibtex-tidy` is skipped with a warning; callers receive notices through `Client.OnWarning`. Formatter and library errors are returned. `FormatExternal` retains its explicit, strict PATH-only API for callers that deliberately want both external tools. The CLI also uses `xclip -selection clipboard`; clipboard failure is reported without losing successful BibTeX output.

The library calls `https://doi.org/<doi>` with `Accept: application/x-bibtex` and follows redirects. It preserves the script's DOI URL, quote, and degree-symbol normalization, and removes all Unicode whitespace (including internal spaces, tabs, line breaks, and nonbreaking spaces) to repair copy artifacts. Invalid DOI input, non-success HTTP responses, unexpected response formats, and failed formatters return errors. Requests time out after 30 seconds; CLI lookup plus formatting has a 60-second deadline.

`doi2bib.Client.Lookup(ctx, doi)` returns formatted, bare BibTeX. `Normalize` validates and normalizes input; `Fenced` produces the note-ready Markdown block. Library calls never access the clipboard or print. `Client.HTTP`, `BaseURL`, and `Format` permit testing or custom integration.

The CLI retains fenced output by default, `--bare`, `--oneshot <file>`, and interactive entry. Lip Gloss styles prompts and diagnostics on stderr; stdout contains only results. Blank lines are skipped in files and end interactive entry. A failed lookup does not stop the remaining DOI lines, but causes a nonzero final exit status. `--help` succeeds without requiring formatter executables. Shell-specific `monkey show GUMINPUTOPTS` customization is not carried over.

No Fyne dependency or app-internal imports are required.
