package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	Version  string
	Revision = "0" 
	CommitId string
)

func main() {
	mustTIOCSTIEnabled()
	var input string

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data is being piped in
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
			os.Exit(1)
		}
		input = string(bytes)
	} else if len(os.Args) > 1 { // Data passed as arguments
		// check if first argument is a flag
		if os.Args[1] == "--version" || os.Args[1] == "-v" {
			// name of the program
			fmt.Printf("%s version %s.%s (%s)\n", os.Args[0], Version, Revision, CommitId)
		}
		input = strings.Join(os.Args[1:], " ")

	}

	if input == "" {
		return
	}

	// for security strip any newlines in the string by matching any newline
	input = string(bytes.ReplaceAll([]byte(input), []byte("\n"), []byte("")))

	f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error tracing /dev/tty: %v\n", err)
		os.Exit(1)
	}
	for _, r := range input {
		// On Linux, the constant is 0x5412
		_, _, err := syscall.Syscall(
			syscall.SYS_IOCTL,
			f.Fd(),
			syscall.TIOCSTI,
			uintptr(unsafe.Pointer(&r)),
		)
		if err != 0 {
			fmt.Fprintf(os.Stderr, "Error: %v offendingg character: %c\n", err, r)
			os.Exit(1)
		}
	}
}

func mustTIOCSTIEnabled() {
	const path = "/proc/sys/dev/tty/legacy_tiocsti"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr,
			"failed to stat sysctl: %w", err)
		os.Exit(1)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read sysctl: %w", err)
		os.Exit(1)
	}
	if bytes.TrimSpace(data)[0] != '1' {
		fmt.Fprintf(os.Stderr, `Error: 'dev.tty.legacy_tiocsti' is disabled in sysctl.\n\tquery with sysctl dev/tty/legacy_tiocsti or cat /proc/sys/dev/tty/legacy_tiocsti\n\ttemporarily change with: sudo sysctl -w dev.tty.legacy_tiocsti=1\n\tpermanently change with: echo "dev.tty.legacy_tiocsti=1" | sudo tee /etc/sysctl.d/99-tiocsti.conf\n`)
		os.Exit(1)
	}
}
