package main

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"io"
	"os"

	"github.com/pydpll/errorutils"
)

// caller is responsible for closing the tmp file
func mirrorFile(arg string) (tmp *os.File, srcSHA string) {
	tmp, err := os.CreateTemp("", "lineExplorer*")
	errorutils.ExitOnFail(err)
	src, err := os.Open(arg)
	errorutils.ExitOnFail(err)
	defer src.Close()
	_, err = io.Copy(tmp, src)
	errorutils.ExitOnFail(err)
	_, err = tmp.Seek(0, io.SeekStart)
	errorutils.ExitOnFail(err)
	_, err = src.Seek(0, io.SeekStart)
	errorutils.ExitOnFail(err)
	hash := md5.New()
	r := bufio.NewReader(src)
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
	srcSHA = fmt.Sprintf("%x", hash.Sum(nil))
	return tmp, srcSHA
}

// function expandTilde handles filenames that include a shorthand reference to the user's home directory
func expandtilde(filename string) string {
	if filename[0] == '~' {
		home, err := os.UserHomeDir()
		errorutils.ExitOnFail(err)
		return home + filename[1:]
	}
	return filename
}
