package main

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pydpll/errorutils"
)

type FileInfo struct {
	Path     string
	Identity string
	IsSym    bool
}

func (f FileInfo) String() string {
	var t string = "file"
	if f.IsSym {
		t = "symlink"
	}
	return fmt.Sprintf("%s: %s", t, f.Path)
}

func run(dirPath, outputPath string) {
	fileInfoChan := make(chan FileInfo, 4094)
	fileRequestChan := make(chan string, 4094)
	var wg sync.WaitGroup
	var wgWrite sync.WaitGroup
	wgWrite.Add(1)
	go func() {
		defer wgWrite.Done()
		f, err := os.Create(outputPath)
		errorutils.ExitOnFail(err, errorutils.WithMsg(fmt.Sprintf("Error opening target file %s", outputPath)))
		defer f.Close()
		writer := bufio.NewWriter(f)
		defer writer.Flush()
		// Write header
		_, err = writer.WriteString("Path\tidentity\tSymlink\n")
		if err != nil {
			fmt.Println("Error writing header:", err)
			return //writer goroutine
		}
		var errorCounter int
		for info := range fileInfoChan {
			if errorCounter > 5 {
				fmt.Fprintf(os.Stderr, "Too many errors, terminating worker")
				return //writer goroutine
			}
			line := fmt.Sprintf("%s\t%s\t%t\n", info.Path, info.Identity, info.IsSym)
			_, err := writer.WriteString(line)
			if err != nil {
				errorCounter++
				errorutils.WarnOnFail(err, errorutils.WithMsg(fmt.Sprintf("error writting %s", info)))
			}
		}
	}()
	wg.Add(1)
	go func(dirPath string, fileRequestChan chan string, wg *sync.WaitGroup) {
		defer wg.Done()
		entries, err := os.ReadDir(dirPath)
		errorutils.WarnOnFail(err, errorutils.WithMsg("couldn't read target dir "+dirPath))

		for _, entry := range entries {
			fullPath := filepath.Join(dirPath, entry.Name())
			if entry.IsDir() {
				err := walkDir(fullPath, fileRequestChan, wg)
				errorutils.WarnOnFail(err, errorutils.WithMsg("Error walking subdirectory:"))
				continue
			}
			fileRequestChan <- fullPath

		}
		if err != nil {
			fmt.Println("Error walking directory:", err)
		}
		close(fileRequestChan)
	}(dirPath, fileRequestChan, &wg)

	for range workersNum {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copyBuf := make([]byte, 1024*1024*512)
			for path := range fileRequestChan {
				fileInfo, err := processFile(path, &copyBuf)
				if err != nil {
					fmt.Println("Error processing file:", path, err)
					return //worker goroutine
				}
				fileInfoChan <- fileInfo
			}
		}()
	}
	wg.Wait()
	close(fileInfoChan)
	wgWrite.Wait()
}

func walkDir(dirPath string, fileRequestChan chan string, wg *sync.WaitGroup) error {
	entries, err := os.ReadDir(dirPath)
	errorutils.WarnOnFail(err, errorutils.WithMsg("couldn't read target dir "+dirPath))

	for _, entry := range entries {
		fullPath := filepath.Join(dirPath, entry.Name())
		if entry.IsDir() {
			err := walkDir(fullPath, fileRequestChan, wg)
			errorutils.WarnOnFail(err, errorutils.WithMsg("Error walking subdirectory:"))
			continue
		}
		fileRequestChan <- fullPath

	}
	return nil
}

func processFile(path string, copyBuf *[]byte) (FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return FileInfo{}, err
	}

	fileInfo := FileInfo{Path: path}
	if info.Mode()&os.ModeSymlink != 0 {
		fileInfo.IsSym = true
		fileInfo.Identity, err = filepath.EvalSymlinks(path)
		if err != nil && os.IsNotExist(err) {
			target, err := os.Readlink(path)
			errorutils.WarnOnFail(err, errorutils.WithMsg("could not grab target for "+path))
			fileInfo.Identity = "BROKEN:" + target
		} else if err != nil {
			errorutils.WarnOnFail(err, errorutils.WithMsg(fmt.Sprintf("%s is Symlink, evaluation failed", path)))
		}
		return fileInfo, nil
	}
	if fileSizeMB := float64(info.Size()) / (1 << 20); fileSizeMB > 90 {
		fileInfo.Identity = "SKIPPED...too big"
		return fileInfo, nil
	}
	result, err := checksum(path, copyBuf)
	if err != nil {
		return fileInfo, err
	}
	fileInfo.Identity = result
	return fileInfo, nil
}

// https://stackoverflow.com/q/60328216/4343913
func checksum(file string, copyBuf *[]byte) (string, error) {
	if IsIgnored(file) {
		return "IGNORED", nil
	}
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}

	defer errorutils.NotifyClose(f)

	h := sha1.New()
	if _, err := io.CopyBuffer(h, f, *copyBuf); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", string(h.Sum(nil))), nil
}
