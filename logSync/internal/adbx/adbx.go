// Package adbx wraps github.com/electricbubble/gadb with the bits logSync
// needs that aren't upstream yet: a per-call timeout (gadb has no
// context.Context support — see ../../docs/upstream-gadb-notes.md item 2)
// and a single-path Stat shim built on List (gadb has no STAT — item 1).
package adbx

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/electricbubble/gadb"
)

// DefaultTimeout bounds every adb round trip issued through this package.
// The bash tool's own history — an adbfs mount going stale and hanging
// every call made through it — is the reason nothing in here is allowed to
// block forever.
const DefaultTimeout = 5 * time.Second

// Connect starts (if needed) and connects to the local adb server.
func Connect() (gadb.Client, error) {
	client, err := gadb.NewClient()
	if err != nil {
		return gadb.Client{}, fmt.Errorf("connect to adb server: %w", err)
	}
	return client, nil
}

// SelectDevice returns the device matching serial, or the sole attached
// device if serial is empty. It errors rather than guessing when more than
// one device is attached and none was requested — the same "don't guess"
// posture taken elsewhere: the bare command has no default mode either.
func SelectDevice(client gadb.Client, serial string) (gadb.Device, error) {
	devices, err := WithTimeoutValue(DefaultTimeout, client.DeviceList)
	if err != nil {
		return gadb.Device{}, fmt.Errorf("list devices: %w", err)
	}
	if serial != "" {
		for _, d := range devices {
			if d.Serial() == serial {
				return d, nil
			}
		}
		return gadb.Device{}, fmt.Errorf("no attached device with serial %q", serial)
	}
	switch len(devices) {
	case 0:
		return gadb.Device{}, fmt.Errorf("no device attached")
	case 1:
		return devices[0], nil
	default:
		return gadb.Device{}, fmt.Errorf("%d devices attached; set device_serial in config to pick one", len(devices))
	}
}

// WithTimeout runs fn and returns its error, or a timeout error if fn hasn't
// returned within d. fn keeps running in the background if it times out
// (gadb gives us no cancellation hook to stop it early — this bounds our own
// wait, not the underlying syscall).
func WithTimeout(d time.Duration, fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
		return fmt.Errorf("timed out after %s", d)
	}
}

// WithTimeoutValue is WithTimeout for functions that also return a value.
func WithTimeoutValue[T any](d time.Duration, fn func() (T, error)) (T, error) {
	type result struct {
		v   T
		err error
	}
	done := make(chan result, 1)
	go func() {
		v, err := fn()
		done <- result{v, err}
	}()
	select {
	case r := <-done:
		return r.v, r.err
	case <-time.After(d):
		var zero T
		return zero, fmt.Errorf("timed out after %s", d)
	}
}

// Stat finds a single entry by listing its parent directory — gadb exposes
// LIST but not the wire protocol's dedicated STAT command (see
// docs/upstream-gadb-notes.md item 1). found is false, err nil when the
// parent directory exists but the entry does not.
func Stat(dev gadb.Device, remotePath string) (entry gadb.DeviceFileInfo, found bool, err error) {
	parent := path.Dir(remotePath)
	name := path.Base(remotePath)
	entries, err := WithTimeoutValue(DefaultTimeout, func() ([]gadb.DeviceFileInfo, error) {
		return dev.List(parent)
	})
	if err != nil {
		return gadb.DeviceFileInfo{}, false, fmt.Errorf("stat %s (via list %s): %w", remotePath, parent, err)
	}
	for _, e := range entries {
		if e.Name == name {
			return e, true, nil
		}
	}
	return gadb.DeviceFileInfo{}, false, nil
}

// DoctorReport is the result of the connectivity and mtime-fidelity
// probe. Whether mtime survives the transfer
// through Android's scoped-storage FUSE layer has been an open question
// since 2026-08-11; MtimeSurvives answers it empirically.
type DoctorReport struct {
	ServerReachable bool
	DeviceSerial    string
	// Dirs reports one line per configured remote directory. Checking only
	// remote_dir used to let doctor pass while notesync failed on a missing
	// vault_remote_dir — the two are separate settings and a device can
	// easily have one and not the other.
	Dirs      []DirCheck
	ProbedDir string
	// FreeBytes is what the device says is left where ProbedDir lives.
	// Best effort: a device whose df output does not parse leaves this
	// unknown rather than failing the check.
	FreeBytes      int64
	FreeKnown      bool
	ProbeRoundTrip bool // bytes survived Push -> Pull unchanged
	MtimeSurvives  bool // List()'s reported mtime matches what we pushed
	Notes          []string
}

// DirCheck is one configured remote directory and whether it is there.
// Key is the config key it came from, so a failure names the line to fix.
type DirCheck struct {
	Key    string
	Path   string
	Exists bool
}

// AllDirsExist reports whether every configured remote directory is
// present. True with no dirs configured — there was nothing to check.
func (r DoctorReport) AllDirsExist() bool {
	for _, d := range r.Dirs {
		if !d.Exists {
			return false
		}
	}
	return true
}

// Doctor runs the pre-flight checks: adb server reachable,
// exactly one usable device, remote dir present, and a real round-trip probe
// file to settle the mtime-fidelity question empirically instead of assuming
// an answer either way.
func Doctor(cfg DoctorConfig) (DoctorReport, error) {
	report := DoctorReport{}

	client, err := Connect()
	if err != nil {
		return report, err
	}
	report.ServerReachable = true

	dev, err := SelectDevice(client, cfg.DeviceSerial)
	if err != nil {
		return report, err
	}
	report.DeviceSerial = dev.Serial()

	if len(cfg.RemoteDirs) == 0 {
		report.Notes = append(report.Notes, "no remote directories configured; skipping mtime probe")
		return report, nil
	}
	for _, d := range cfg.RemoteDirs {
		_, found, err := Stat(dev, d.Path)
		if err != nil {
			return report, fmt.Errorf("checking %s: %w", d.Key, err)
		}
		report.Dirs = append(report.Dirs, DirCheck{Key: d.Key, Path: d.Path, Exists: found})
		if !found {
			report.Notes = append(report.Notes, fmt.Sprintf("%s %s does not exist (yet)", d.Key, d.Path))
		}
	}
	// The probe only needs somewhere writable on the device; run it in the
	// first directory that is actually there rather than giving up because
	// a different one is missing.
	for _, d := range report.Dirs {
		if d.Exists {
			report.ProbedDir = d.Path
			break
		}
	}
	if report.ProbedDir == "" {
		return report, nil
	}

	if free, ok := freeSpace(dev, report.ProbedDir); ok {
		report.FreeBytes, report.FreeKnown = free, true
	} else {
		report.Notes = append(report.Notes, "could not read free space on the device")
	}

	probeName, err := randomProbeName()
	if err != nil {
		return report, fmt.Errorf("generating probe filename: %w", err)
	}
	remoteProbe := path.Join(report.ProbedDir, probeName)
	wantMtime := time.Now().Truncate(time.Second)
	content := []byte("logSync doctor probe\n")

	err = WithTimeout(DefaultTimeout, func() error {
		return dev.Push(bytes.NewReader(content), remoteProbe, wantMtime)
	})
	if err != nil {
		return report, fmt.Errorf("push probe file: %w", err)
	}
	// Best-effort cleanup even if later checks fail.
	defer func() {
		_ = WithTimeout(DefaultTimeout, func() error {
			_, err := dev.RunShellCommand("rm", "-f", shellQuote(remoteProbe))
			return err
		})
	}()

	entries, err := WithTimeoutValue(DefaultTimeout, func() ([]gadb.DeviceFileInfo, error) {
		return dev.List(report.ProbedDir)
	})
	if err != nil {
		return report, fmt.Errorf("list after push: %w", err)
	}
	var pushedMtime time.Time
	var sawPushed bool
	for _, e := range entries {
		if e.Name == probeName {
			pushedMtime = e.LastModified
			sawPushed = true
			break
		}
	}
	if !sawPushed {
		report.Notes = append(report.Notes, "pushed probe file did not appear in a subsequent LIST")
		return report, nil
	}
	report.MtimeSurvives = pushedMtime.Equal(wantMtime)
	if !report.MtimeSurvives {
		report.Notes = append(report.Notes, fmt.Sprintf("mtime drift: pushed %s, device reports %s", wantMtime, pushedMtime))
	}

	var buf bytes.Buffer
	err = WithTimeout(DefaultTimeout, func() error {
		return dev.Pull(remoteProbe, &buf)
	})
	if err != nil {
		return report, fmt.Errorf("pull probe file back: %w", err)
	}
	report.ProbeRoundTrip = bytes.Equal(buf.Bytes(), content)
	if !report.ProbeRoundTrip {
		report.Notes = append(report.Notes, "pulled probe content did not match what was pushed")
	}

	return report, nil
}

// DoctorConfig is the subset of Config Doctor needs, kept separate so
// internal/adbx does not import the main package.
type DoctorConfig struct {
	DeviceSerial string
	RemoteDirs   []RemoteDir
}

// RemoteDir names one configured device directory to check, paired with
// the config key it came from.
type RemoteDir struct {
	Key  string
	Path string
}

// freeSpace asks the device how much room is left where dir lives. A large
// notesync can otherwise fail partway through with a scatter of per-file
// errors and no hint that the disk was simply full.
func freeSpace(dev gadb.Device, dir string) (bytes int64, ok bool) {
	out, err := WithTimeoutValue(DefaultTimeout, func() (string, error) {
		return dev.RunShellCommand("df", "-k", shellQuote(dir))
	})
	if err != nil {
		return 0, false
	}
	// Android's df prints a header then one row; the available column is
	// the fourth, in 1K blocks.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		fields := strings.Fields(lines[i])
		if len(fields) < 4 {
			continue
		}
		kb, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			continue
		}
		return kb * 1024, true
	}
	return 0, false
}

func randomProbeName() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return ".logSync-probe-" + hex.EncodeToString(b), nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
