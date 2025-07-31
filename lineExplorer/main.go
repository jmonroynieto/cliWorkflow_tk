package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"unicode/utf8"

	"golang.org/x/term"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	CommitId string
	Version  string
	Revision = ".1"
)

func main() {
	defer retainFunctionalTTY(termSttyState())
	err := app.Run(context.Background(), os.Args)
	errorutils.ExitOnFail(err)
}

var app *cli.Command = &cli.Command{
	Name:    "lineExplorer",
	Version: fmt.Sprintf("%s%s (%s)", Version, Revision, CommitId),
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"d"},
			Usage:   "activates debugging messages",
			Action: func(ctx context.Context, cmd *cli.Command, shouldDebug bool) error {
				if shouldDebug {
					logrus.SetLevel(logrus.DebugLevel)
				}
				return nil
			},
		},
	},
	Action: readAndShow,
}

func readAndShow(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) == 0 {
		return errorutils.NewReport("ERROR: no files provided", "VLsmXBwQrya")
		//TODO: think about piping in data
	}
	for _, arg := range args {
		ogFilepath := expandtilde(arg)
		tmp, srcSHA := mirrorFile(ogFilepath)
		idx := indexLines(tmp)
		model := New(tmp, idx)
		//displays a tui using bubbletea with a random line and the lines sorrounding it
		// The user has several options: they can reshuffle lines, navigate through the vecity of lines to identify and mark lines for potential deletion. She may select and delete multiple lines. Reshuffling  keeps a list of deleted lines that will be applied to the file once the user submits. If the user aborts, no changes are applied.
		changes, err := tea.NewProgram(model).Run()
		if err != nil {
			return err
		}	
		//once it is over, the file is overwritten without the lines indicated by the user
		if logrus.IsLevelEnabled(logrus.DebugLevel) { //no changes
			logrus.Debugf("changes: %v", changes.(Model).shouldDelete)
			return nil
		}
		if len(changes.(Model).shouldDelete) == 0 {
			return nil // no changes to save to the original file
		}
		err = applyChanges(tmp, ogFilepath, changes.(Model).shouldDelete, srcSHA)
		if err != nil {
			return err
		}
	}
	return nil
}

type index []uint32 // index[i] Reads as "the offset to the newline at the end of the ith line  and therefore, len(index)-1 is a line count since the first element is always 0.

func indexLines(r io.Reader) index {
	var idx index = make([]uint32, 0, 32768) // 128kb
	var pos uint32
	idx = append(idx, pos) //first element is always 0
	var x sync.Once
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		shouldContinueOn := true
		x.Do(func() {
			if !utf8.ValidString(scanner.Text()) {
				logrus.Warn("File contains invalid UTF-8 characters")
				shouldContinueOn = false
			}
		})
		if !shouldContinueOn {
			break //skip file
		}
		pos += uint32(len(scanner.Bytes())) + 1
		idx = append(idx, pos)
	}
	errorutils.ExitOnFail(scanner.Err())
	result := make(index, len(idx))
	copy(result, idx)
	return result
}

// startLine and endLine are 1 based
func (i index) readlines(file *os.File, startLine uint32, endLine uint32) ([]string, error) {
	if endLine > uint32(len(i))-1 {
		logrus.Warn("endLine is out of range, setting to end of file")
		endLine = uint32(len(i))
	}
	if startLine <= 0 || endLine <= 0 || startLine > endLine {
		return nil, fmt.Errorf("invalid line range: startLine=%d, endLine=%d", startLine, endLine)
	}
	buf := make([]byte, i[endLine]-(i[max(startLine-1, 0)]))
	file.ReadAt(buf, int64(i[max(startLine-1, 0)]))
	if buf[len(buf)-1] == '\x00' || buf[len(buf)-1] == '\n' { // handle final newline; either missing (aka POSIX malformed) or present and needs to be stripped due to split artifact
		buf = buf[:len(buf)-1]
	}
	result := make([]string, 0, endLine-startLine+1)
	for _, line := range bytes.Split(buf, []byte("\n")) {
		result = append(result, string(line))
	}
	return result, nil
}

func applyChanges(tmp *os.File, target string, shouldDelete map[uint32]struct{}, srcSHA string) error {
	{ // check if target file has been modified
		ogFile, err := os.Open(target)
		errorutils.ExitOnFail(err)
		r := bufio.NewReader(ogFile)
		hash := md5.New()
		buf := make([]byte, 1024)
		for {
			n, err := r.Read(buf)
			if err != nil && err != io.EOF {
				errorutils.ExitOnFail(err)
			}
			if n == 0 {
				break
			}
			hash.Write(buf[:n])
			if err == io.EOF {
				break
			}
		}
		tSHA := fmt.Sprintf("%x", hash.Sum(nil))
		if tSHA != srcSHA {
			validateOverwrite.Run()
			if !userOverwrite {
				return errorutils.NewReport("user requested modification to the file, but these were not saved. Your changes are stashed into "+tmp.Name(), "tFMpyFIa4FH")
			}
		}
	}

	//overwrite
	_, err := tmp.Seek(0, io.SeekStart)
	errorutils.ExitOnFail(err)
	scratchpad, err := os.CreateTemp("", "lineExplorerDel*")
	w := bufio.NewWriter(scratchpad)
	scanner := bufio.NewScanner(tmp)
	lineCounter := uint32(1)
	for scanner.Scan() {
		if _, ok := shouldDelete[lineCounter]; ok {
			lineCounter++
			continue
		}
		w.WriteString(scanner.Text())
		w.WriteString("\n")
		lineCounter++
	}
	errorutils.ExitOnFail(scanner.Err())
	err = w.Flush()
	errorutils.ExitOnFail(err)
	err = os.Remove(tmp.Name())
	errorutils.ExitOnFail(err)
	//preserve permissions
	info, err := os.Stat(target)
	errorutils.ExitOnFail(err)
	err = os.Chmod(scratchpad.Name(), info.Mode())
	errorutils.ExitOnFail(err)
	//rename scratchpad to target
	err = os.Rename(scratchpad.Name(), target)
	if err != nil {
		//handle filesystem boundary errors and copy instead
		if errors.Is(err, syscall.EXDEV) {
			logrus.Debug("unable to rename scratchpad to target, copying instead")
			src, err := os.Open(scratchpad.Name())
			errorutils.ExitOnFail(err)
			defer src.Close()
			dst, err := os.Create(target)
			errorutils.ExitOnFail(err)
			defer dst.Close()
			_, err = io.Copy(dst, src)
			errorutils.ExitOnFail(err)
			err = os.Remove(scratchpad.Name())
			errorutils.ExitOnFail(err)
		}
	}
	return nil
}

var userOverwrite bool

func retainFunctionalTTY(fd uintptr, oldState *term.State) {
	defer term.Restore(int(fd), oldState)
	if v := recover(); v != nil {
		logrus.Warn(v)
	}

}

func termSttyState() (uintptr, *term.State) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		panic(err)
	}
	defer tty.Close()
	fd := int(tty.Fd())
	oldState, err := term.GetState(int(fd))
	errorutils.ExitOnFail(err)
	return uintptr(fd), oldState
}
