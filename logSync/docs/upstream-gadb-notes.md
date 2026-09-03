# Upstream notes for github.com/electricbubble/gadb

logSync depends on `gadb` (v0.1.0) to talk to the `adb` server's sync
protocol directly instead of shelling out to the `adb` CLI per file. It's the
healthier of the two commonly-cited Go adb libraries — `zach-klippenstein/goadb`
has had no commit since 2020-12-08 and, notably, **has no `go.mod` at all**
(confirmed via its own open issue "Use of internal package .../internal/errors
not allowed"), so it isn't consumable as a normal module without forking one
in. `gadb` last committed 2025-03-14 and ships a `go.mod`.

These are gaps found while building `internal/adbx` as a wrapper around it.
None of them block logSync (the wrapper works around each), but each is a
plausible, scoped contribution back — evidence and a suggested fix included
so whoever picks one up doesn't have to re-derive it.

## 1. No `Stat` — only `List`

The adb sync wire protocol has a dedicated `STAT` command for single-path
metadata. `gadb` only exposes `Device.List(dir)` (directory listing);
checking one file's metadata means listing its whole parent directory and
scanning for a matching name (`internal/adbx.Stat` in this repo does exactly
that as a stopgap). A `Device.Stat(path) (DeviceFileInfo, error)` sending
`STAT` directly would be both simpler for callers and cheaper for large
directories.

## 2. No `context.Context` anywhere

`Device.List`, `.Push`, `.Pull`, and `.RunShellCommand` all block on the
underlying socket with only a fixed `DefaultAdbReadTimeout` (60s) as a
backstop — no way to pass a shorter deadline or cancel in flight. This is a
plausible root cause for the repo's own open issue, **"some shell cmd frozen
without return"** — without cancellation, a caller has no way to bound or
recover from a hung command. `internal/adbx.WithTimeout`/`WithTimeoutValue`
in this repo work around it at the call-site by racing the call against a
timer, but the goroutine calling into `gadb` keeps running regardless (it
can't be told to stop) — a real fix needs `context.Context` threaded down to
the socket read/write calls in `transport.go` and `syncTransport`.

## 3. No adb shell protocol v2 (stdout/stderr split, real exit code)

`RunShellCommand`/`RunShellCommandWithBytes` return one merged byte blob with
no exit status. This is a long-standing, near-identical request already
filed against the sibling library `goadb` ("Support shell out/err splitting
and exit codes") — it's an ecosystem-wide gap in the Go adb space, not
specific to `gadb`. Not needed in the sync path (that's the point of using
`sync:` directly instead of `adb shell tar ...`), but `primeMobile` runs
`mkdir -p` remotely and has to infer failure from text on stdout — see
`upstream-gadb-contributions.md` for how that plays out. Worth fixing once,
upstream, rather than every consumer inventing its own workaround.

## 4. `DeviceFileInfo.Mode` is a POSIX `st_mode` typed as `os.FileMode`

```go
func (info DeviceFileInfo) IsDir() bool {
	return (info.Mode & (1 << 14)) == (1 << 14)
}
```

This tests POSIX's `S_IFDIR` bit position (`0040000`, bit 14) against a value
typed as `os.FileMode`. Go's `os.FileMode` does **not** use POSIX's bit
layout for its high mode bits — `os.ModeDir` and friends live at different
positions. If the raw wire `mode` `uint32` (a real POSIX `st_mode`) is being
stored directly into a field typed `os.FileMode` without translating POSIX
bits into Go's `os.FileMode` bits first, `IsDir()` is checking the wrong
thing for at least some values — which lines up with `gadb`'s own open issue,
**"IsDir is not work well."** **Confirmed against a real device on 2026-09-02, and the diagnosis above is
wrong**: `IsDir()` itself is right, because the raw `st_mode` is stored
unchanged and POSIX bit 14 really is set (a directory lists as `042770`).
What breaks is every use of the field *as* an `os.FileMode` — `String()`
renders a directory as a regular file, and `info.Mode.IsDir()` is false for
everything. The defect is the field's type, not the bit test. See
`upstream-gadb-contributions.md`.

## 5. `DeviceFileInfo.Size` is `uint32` (low priority)

A 4 GB ceiling, inherited from the sync protocol's original `LIST`/`STAT`
wire format (`LIST_V2`/`STAT_V2` in newer adb use 64-bit sizes). Irrelevant
to logSync's note-sized files; noted only as a known limitation for anyone
using `gadb` for larger transfers.

---

None of the above have been filed as GitHub issues yet — intentionally left
for the user to file (or not) under their own account.

Items 1-3 and 5 remain read-the-code inferences, cross-referenced against
gadb's and goadb's own issue trackers. Item 4 was one too until 2026-09-02,
when a device was finally attached and turned it into a measurement — and
into a different bug than the one guessed at. Treat the rest with the same
suspicion until something similar happens to them.

`upstream-gadb-contributions.md` carries what the device has confirmed so
far, and the workarounds in `internal/adbx` that each gap produced.
