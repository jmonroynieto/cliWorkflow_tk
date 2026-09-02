package adbx

import (
	"fmt"
	"strings"
)

// PrimeResult is what happened to one configured remote directory.
type PrimeResult struct {
	Key     string
	Path    string
	Created bool // false when it was already there
}

// Prime creates the configured remote directories on the device. It only
// ever touches the device side: the local counterparts are the user's own
// tree, and a tool that silently conjures a local_dir would make a typo in
// the config look like a working setup rather than the mistake it is.
//
// Creating a directory that already exists is not an error — the command is
// meant to be safe to re-run after doctor reports a new missing path.
func Prime(cfg DoctorConfig) ([]PrimeResult, error) {
	client, err := Connect()
	if err != nil {
		return nil, err
	}
	dev, err := SelectDevice(client, cfg.DeviceSerial)
	if err != nil {
		return nil, err
	}

	var results []PrimeResult
	for _, d := range cfg.RemoteDirs {
		if !strings.HasPrefix(d.Path, "/") {
			return results, fmt.Errorf("%s must be an absolute device path, got %q", d.Key, d.Path)
		}
		_, found, err := Stat(dev, d.Path)
		if err != nil {
			// A missing parent makes Stat's LIST fail; that is exactly the
			// case mkdir -p is for, so fall through rather than give up.
			found = false
		}
		if found {
			results = append(results, PrimeResult{Key: d.Key, Path: d.Path})
			continue
		}
		out, err := WithTimeoutValue(DefaultTimeout, func() (string, error) {
			return dev.RunShellCommand("mkdir", "-p", shellQuote(d.Path))
		})
		if err != nil {
			return results, fmt.Errorf("creating %s %s: %w", d.Key, d.Path, err)
		}
		// adb shell reports mkdir failures on stdout with a zero exit
		// status, so the only evidence of "permission denied" or "read-only
		// file system" is the text itself.
		if msg := strings.TrimSpace(out); msg != "" {
			return results, fmt.Errorf("creating %s %s: %s", d.Key, d.Path, msg)
		}
		if _, found, err := Stat(dev, d.Path); err != nil || !found {
			return results, fmt.Errorf("creating %s %s: directory still absent after mkdir -p", d.Key, d.Path)
		}
		results = append(results, PrimeResult{Key: d.Key, Path: d.Path, Created: true})
	}
	return results, nil
}
