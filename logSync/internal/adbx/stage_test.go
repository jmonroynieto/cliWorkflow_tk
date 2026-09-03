package adbx

import (
	"testing"
	"time"
)

// A flat 30 s budget per file timed out any attachment past roughly 400 MB
// on a link that was working fine (a USB-attached tablet moved 40 MB in
// 2.9 s). The deadline has to grow with the file while still bounding a
// transfer that is genuinely stuck.
func TestTransferTimeout(t *testing.T) {
	if got := transferTimeout(0); got != StageTimeout {
		t.Errorf("empty file should get the floor, got %s", got)
	}
	if got := transferTimeout(1 << 20); got != StageTimeout+time.Second {
		t.Errorf("1 MB should add one second, got %s", got)
	}
	if got := transferTimeout(500 << 20); got <= 8*time.Minute {
		t.Errorf("a 500 MB file needs minutes, not %s", got)
	}
	if got := transferTimeout(-1); got != StageTimeout {
		t.Errorf("a nonsense size must not shorten the deadline, got %s", got)
	}
}

// The size+mtime shortcut cannot tell a same-length, same-second edit apart
// from an unchanged file, so it is only worth taking where the download it
// avoids is actually expensive.
func TestShortcutOnlyAppliesToLargeFiles(t *testing.T) {
	if shortcutMinSize <= 0 {
		t.Fatal("a zero threshold would trust size+mtime for every note")
	}
	small := int64(4 << 10) // a typical note
	if small >= shortcutMinSize {
		t.Errorf("a %d-byte note must always be fetched, not trusted", small)
	}
	big := int64(40 << 20) // the attachment the shortcut exists for
	if big < shortcutMinSize {
		t.Errorf("a %d-byte attachment should still take the shortcut", big)
	}
}
