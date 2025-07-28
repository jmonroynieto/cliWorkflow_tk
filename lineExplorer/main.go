package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"sync"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	CommitId string
	Version  string
	Revision = ".0"
)

func main() {
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
		x := expandtilde(arg)
		tmp, srcSHA := mirrorFile(x)
		idx := indexLines(tmp)
		model := New(tmp, idx)
		//displays a tui using bubbletea with a random line and the lines sorrounding it
		// The user has several options: they can reshuffle lines, navigate through the file to identify and mark lines for potential deletion, delete the current line, or select and delete multiple lines. These actions can help manage repeated or slightly modified lines efficiently.
		changes, err := tea.NewProgram(model).Run()
		if err != nil {
			return err
		}
		//once it is over, the file is overwritten without the lines indicated by the user
		if logrus.IsLevelEnabled(logrus.DebugLevel) { //no changes
			logrus.Debugf("changes: %v", changes.(Model).shouldDelete)
			return nil
		}
		err = applyChanges(tmp, x, changes.(Model).shouldDelete, srcSHA)
		if err != nil {
			return err
		}
	}
	return nil
}

type index []uint32 // index[i] Reads as "the newline at the end line i and therefore, len(index) is a line count. First element is always 0

func indexLines(r io.Reader) index {
	var idx index = make([]uint32, 0, 32768) // 128kb
	var pos uint32
	idx = append(idx, pos) // only index point at the beginning of a line.
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
			break //skip
		}
		pos += uint32(len(scanner.Bytes()))
		idx = append(idx, pos)
		pos += 1

	}
	errorutils.ExitOnFail(scanner.Err())
	result := make(index, len(idx))
	copy(result, idx)
	return result
}

// startLine and endLine are 1 based
func (i index) readlines(file *os.File, startLine uint32, endLine uint32) ([]string, error) {
	if endLine > uint32(len(i)) {
		logrus.Warn("endLine is out of range, setting to end of file")
		endLine = uint32(len(i))
	}
	if startLine <= 0 || endLine <= 0 || startLine > endLine {
		return nil, fmt.Errorf("invalid line range: startLine=%d, endLine=%d", startLine, endLine)
	}
	var newlineskip uint32 = 1
	if startLine == 1 {
		newlineskip = 0
	}
	buf := make([]byte, i[endLine]-(i[startLine-1]+newlineskip))
	file.ReadAt(buf, int64(i[startLine-1]+newlineskip))

	result := make([]string, 0, endLine-startLine+1)
	for _, line := range bytes.Split(buf, []byte("\n")) {
		result = append(result, string(line))
	}
	return result, nil
}

func applyChanges(tmp *os.File, target string, shouldDelete map[uint32]struct{}, srcSHA string) error {
	t, err := os.Open(target)
	errorutils.ExitOnFail(err)
	{ // check if target file has been modified
		r := bufio.NewReader(t)
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
		}
		tSHA := fmt.Sprintf("%x", hash.Sum(nil))
		if tSHA != srcSHA {
			return errorutils.NewReport("ERROR: target file has been modified", "tFMpyFIa4FH")
		}
	}

	//overwrite
	_, err = tmp.Seek(0, io.SeekStart)
	errorutils.ExitOnFail(err)
	//truncate target
	err = t.Truncate(0)
	errorutils.ExitOnFail(err)
	w := bufio.NewWriter(t)
	scanner := bufio.NewScanner(tmp)
	lineCounter := uint32(1)
	for scanner.Scan() {
		if _, ok := shouldDelete[lineCounter]; ok {
			continue
		}
		w.WriteString(scanner.Text())
		w.WriteString("\n")
		lineCounter++
	}
	errorutils.ExitOnFail(scanner.Err())
	err = w.Flush()
	errorutils.ExitOnFail(err)
	return nil
}
