package main

import (
  "fmt"
  "os"
  "time"
  "sync"
  "github.com/fsnotify/fsnotify"
)

func main() {
	// Get the requested time in seconds from the command line argument.
	requestedTime, err := time.ParseDuration(os.Args[1])
	if err != nil {
		fmt.Println(err)
		return
	}

	// Get the current directory.
	currDir, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Create a lock to protect the directory.
	lock := &sync.Mutex{}

	// Watch the current directory for new files.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer watcher.Close()

	// Add the current directory to the watcher.
	err = watcher.Add(currDir)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Start a goroutine to watch for new files.
	go func() {
		for {
			// Lock the directory.
			lock.Lock()
			defer lock.Unlock()

			// Watch for events.
			for event := range watcher.Events {
				if event.Op == fsnotify.Create {
					// A new file was created.
					fmt.Println("A new file named", event.Name, "was created at", time.Now().Format("January 2, 2006 at 15:04:05"))
				}
			}
		}
	}()

	// Wait for the specified amount of time.
	time.Sleep(requestedTime)
}
