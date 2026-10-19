# bibstomp

`bibstomp` is a package inside the `doi2bib` module. It exposes BibTeX
parsing, name handling, Unicode folding, and citation-key minting as package
`bibstomp`. Its optional command is in `cmd/bibstomp`:

```sh
go test ./...
go build -o bibstomp ./cmd/bibstomp
./bibstomp stomp < references.bib
```

Library use:

```go
result, err := bibstomp.Stomp(ctx, bibtex)
```

`Stomp` uses the command's default `{authors}_{year}` key policy and preserves
field contents. It reports parser warnings as errors, preserves duplicate minted
keys (no invented suffixes), and uses `Anon`/`0000` for missing authors/years.
`ParseBib`, `MintKey`, `DefaultKeyOpts`, and `ApplySplices` expose the underlying
operations and diagnostics for callers needing custom templates or review.
The library does not read PATH, write files, or launch a process. Concurrent
calls are supported.

`doi2bib` uses this package directly when it remints citation keys.
