# Known limitations and open decisions

logSync syncs an Obsidian vault between a Linux machine and an Android
phone over adb. It replaces an earlier bash implementation of the same
workflow, and inherits that tool's merge semantics deliberately. This file
records what is known not to work, or not to be decided, as of the device
testing described below.

Everything here was exercised against a real phone, not only in unit tests.

## Merge behaviour

### A note's first two-sided merge has no ancestor unless it was seeded

Merging is a 3-way merge when an ancestor is available and a union against
an empty base when it is not, and the difference is visible in the result:
an empty base re-emits whatever the two sides already shared. Three sources
of ancestors exist — a git tracking repo, a baseline recorded by a previous
converged round, and a baseline recorded when `notesync` seeded the device.
A file that has none of those, on a machine pair that has never converged
it, gets the union.

For notes the cost is duplicated lines. For JSON — every file under the
vault's configuration directory, and every plugin's settings — the cost is
a file that no longer parses, because duplicated keys and a dropped comma
are not valid JSON. That is the main reason the configuration directory is
excluded by default; see below.

### A genuine conflicting edit is silently duplicated

Two edits to the same line produce both lines, one after the other, with
nothing said. `merge.Into` runs `git merge-file -p --union`, and `--union`
resolves every hunk by keeping both sides, so the conflict count it returns
is structurally always 0. The caller then discards the result. Detection is
dead twice over.

Nothing below is implemented. These are the options, with what each costs.

**The step that unblocks all of them.** Run `git merge-file` a second time
on the same three inputs *without* `--union`, purely to read its exit
status, while the `--union` pass goes on producing what is written to disk.
The exit status is the count of overlapping hunks. One extra local process
per differing file — a handful per round, none at all on a round with no
differences — and not one byte of output changes. Every option after this
depends on it.

**The caveat that shapes the rest.** Detection is only meaningful when the
merge had a real ancestor. Against an empty base every hunk overlaps by
definition, so a detection pass on a file that has no baseline yet would
report a conflict for *every* differing file — and that is the common case
on a first run. A report has to be gated on `Result.UsedBase`, which
`merge.Into` already returns, or the first run after seeding a device
drowns in conflicts that are not conflicts.

Once a conflict is known:

- *Report it, change nothing.* Print the count per file and let the union
  result stand. Costs nothing beyond the detection pass and needs no new
  files or configuration. But someone has to go and read the note, and if
  they do not, the duplicated lines stay.
- *Write a sidecar.* Put the marked-up output in a sibling file —
  `note.md.conflict`, deliberately not `.md` so Obsidian does not index it
  as a note — while the note itself receives the clean union result.
  Nothing is broken and nothing is hidden. The sidecars need excluding from
  the sync, which `exclude_path`'s globs express as `**/*.conflict`, and
  they accumulate until someone deletes them.
- *Write conflict markers into the note.* The standard git outcome: the
  note cannot be left alone. It is the only option that guarantees the
  conflict is dealt with, and the only one that puts a broken note in a
  vault that is read on a phone — the next push sends the broken version
  there. Adding `--diff3` includes the base region between the markers,
  which makes resolution much easier at the cost of a longer block.
- *Refuse to write the file.* Leave both sides untouched at that path,
  report it, carry on with every other file, exit non-zero. Safest for
  content, because nothing is merged at all. The two sides stay divergent
  until a human intervenes, and every later round detects it again.
- *Make it a setting.* `on_conflict = report | sidecar | markers | skip`.
  More surface to maintain, and it postpones the decision rather than
  making it — but the right answer plausibly differs between a small
  append-heavy merge scope and a whole vault.

**One asymmetry worth knowing before choosing.** If a conflicted file is
merged by union and pushed, both sides end up byte-identical, so the round
records that union result as the new baseline and the conflict is
forgotten — it will never be reported again, because the next round starts
from an ancestor that already contains both versions. "Report and move on"
is therefore a one-shot warning. Only an option that declines to record
convergence will keep nagging.

### Two edits on adjacent lines cannot be told apart

With no unchanged line between them, diff3 cannot distinguish two
independent single-line edits from one edit spanning both, and under
`--union` that degrades to duplication rather than a marked conflict. This
is a general limit of line-based 3-way merging, not something this tool can
fix. `sync_test.go` documents it as a boundary, with a `t.Skip` that fires
if git's alignment ever improves enough to make the test obsolete.

### Binary content is never merged, and local wins

An image, font or PDF that differs on both sides is sent from here to the
phone rather than merged — a union of two PNGs is not a PNG. That means a
change made to the same binary on the phone is discarded, and unlike a text
conflict there is no half-way result to inspect afterwards. Whether that is
the right policy is part of the conflict decision above.

## The vault configuration directory

`.obsidian/` is excluded unless `--with-obsidian` is passed, and even then
nothing inside it is written by an ordinary run: differences are printed
with a diff for review. The one sanctioned write is `--bubble <path>`,
which copies a named file from the phone over the local copy.

This is deliberate rather than provisional. Obsidian rewrites several of
those files every time it starts, so they become merge candidates
constantly; they are JSON, which a union without a baseline turns into
something that does not parse; and the directory is most of a vault by file
count once plugin code, fonts and themes are counted.

What has no sanctioned path yet is the other direction: pushing a
configuration change from here to the phone. The assumption is that the
base configuration already on the phone is enough for the work done there.
If that stops being true, it needs a decision rather than a default.

## Deletions

A file present on one side and absent on the other is ambiguous — new and
not yet synced, or deleted on the other side. Paths tracked in the git
repo, where a deletion is a real possibility, prompt; untracked ones are
assumed new and copied.

Nothing is ever deleted on either side by default. Answering "yes, it was
deleted" means only "do not copy it back", so the file stays where it is
and the next run asks again. There is no memory of the answer.

`sync --prune` and `push --prune` are the opt-in exception, for scopes like
`LOGFILE/` that are deliberately outside git and so have no history to
disambiguate against: every phone-only file otherwise reads as "new" and
gets copied back regardless of whether it was actually removed locally on
purpose. With `--prune`, local's absence is instead taken as fact — the
file is deleted from the phone, no prompt, whether or not it happens to be
git-tracked. The failure mode this trades for: a note created directly on
the phone and never yet pulled down looks identical to one deleted locally,
and `--prune` deletes it. It is per-invocation, not a config default, for
that reason. `pull` has no such flag — phone-only files there are the
copy-in target, not a deletion candidate.

## The snapshot store

Every file that converges leaves a full copy of its content under
`$XDG_STATE_HOME/logSync/snapshot`, keyed by its absolute local path. That
is a second place vault content lives on the machine, outside the vault and
outside any repo, which matters if some of it is sensitive.

A sync round prunes the baselines for files it found gone from both sides,
within the directory it was syncing. It will not touch anything when the
sync root itself is absent, because a vault whose mount is not up looks
exactly like a deleted one. Those cases, and baselines left by a vault that
really did move, are settled by `logSync snapshot prune --orphans` after
reading what it lists.

Moving a vault orphans every baseline under it, silently returning the tool
to first-merge behaviour for every file. Nothing detects or reports that
beyond `logSync snapshot`.

## Transport

### Metadata is trusted for large files

Staging skips the download when a remote file's size and mtime match the
local copy, but only for files of 1 MiB or more. Below that the bytes are
always fetched, because two small files of the same length written within
the same second are not necessarily the same file, and a note edited on
both devices in one second is a plausible way to reach that. Above it the
same risk remains and is the price of not moving a large attachment on
every run. Measured cost of always fetching the small ones: roughly 5.5 ms
per file.

### Some filenames the vault allows, the device refuses

Android's storage layer rejects `" * : < > ? \ |` in filenames. A note
whose name contains one cannot be pushed; the transfer names the file and
the reason and the run exits non-zero, but there is no way to sync it short
of renaming it.

### A staging failure is reported, not retried

A file the device lists but will not hand over is carried as "phone state
unknown" for the rest of the round: it is dropped from the one-sided
buckets so it cannot be mistaken for a deletion, and its baseline is kept.
It is not retried, and the run does not fail because of it.

## Diagnostics

`doctor` checks that the adb server is reachable, that each configured
remote directory exists, that a probe file survives a push/pull round trip
unchanged, that its mtime survives, and how much room is left on the
device. It does not check writability beyond the probe, so a large
`notesync` can still fail partway through on a full disk.

## Documentation

Comments throughout refer to "the bash tool" — the earlier implementation
this replaces. Its design documents are not part of this repository, and
the comments have been written to stand without them. Where a claim came
from one of those documents it has been restated here rather than cited.
