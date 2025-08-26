package main

import (
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// styling parameters for button-like items
type ButtonConfig struct {
	HorizontalPadding int
	ItemMargin        int
	BorderWidth       int
	MinWidth          int
}

func DefaultButtonConfig() ButtonConfig {
	return ButtonConfig{
		HorizontalPadding: 2, // 1 space on each side
		ItemMargin:        1, // 1 space between items
		BorderWidth:       1, // single border
		MinWidth:          0, // no minimum width
	}
}

// Total width an item will take including styling
func (c ButtonConfig) CalculateItemWidth(content string) int {
	contentWidth := len(content)
	if c.MinWidth > contentWidth {
		contentWidth = c.MinWidth
	}
	totalWidth := contentWidth + c.HorizontalPadding + (c.BorderWidth * 2)
	return totalWidth
}

func GetTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80
	}
	return width
}

func portionItemsToLines(items []string, config ButtonConfig, terminalWidth int) [][]string {
	if len(items) == 0 {
		return [][]string{}
	}

	var result [][]string
	var currentLine []string
	currentLineWidth := 0

	for _, item := range items {
		item = strings.TrimSpace(item)
		itemWidth := config.CalculateItemWidth(item)

		// Calculate the width this item would add to the current line
		widthToAdd := itemWidth
		if len(currentLine) > 0 {
			// Add margin if this isn't the first item on the line
			widthToAdd += config.ItemMargin
		}

		// Check if adding this item would exceed terminal width
		if currentLineWidth+widthToAdd >= terminalWidth {
			// Start a new line
			if len(currentLine) > 0 {
				result = append(result, currentLine)
			}
			currentLine = []string{item}
			currentLineWidth = itemWidth
		} else {
			// Add to current line
			currentLine = append(currentLine, item)
			currentLineWidth += widthToAdd
		}
	}

	// Don't forget the last line
	if len(currentLine) > 0 {
		result = append(result, currentLine)
	}

	return result
}

// WrapItemsToLinesAuto is a convenience function that auto-detects terminal width
func WrapItemsToLinesAuto(items []string, config ButtonConfig) [][]string {
	return portionItemsToLines(items, config, GetTerminalWidth())
}

type RollingBuffer struct {
	mu       sync.RWMutex
	buf      []string
	start    int // index of the first valid item
	size     int // number of valid items
	capacity int
}

// NewRollingBuffer allocates a new ring buffer of given capacity
func NewRollingBuffer(cap int) *RollingBuffer {
	return &RollingBuffer{
		buf:      make([]string, 0, cap),
		capacity: cap,
	}
}

// Add appends item—if full, it overwrites the oldest and moves start forward
func (rb *RollingBuffer) Add(item string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size < rb.capacity {
		if len(rb.buf) <= rb.size {
			rb.buf = append(rb.buf, item)
		} else {
			rb.buf[rb.size] = item
		}
		rb.size++
		return
	}
	// overwrite oldest
	rb.start = (rb.start + 1) % rb.capacity
	if len(rb.buf) <= rb.start {
		rb.buf = append(rb.buf, item)
	} else {
		rb.buf[rb.start] = item
	}
}

// GetAll returns a snapshot of current items in insertion order
func (rb *RollingBuffer) GetAll() []string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	out := make([]string, rb.size)
	for i := 0; i < rb.size; i++ {
		out[i] = rb.buf[(rb.start+i)%len(rb.buf)]
	}
	return out
}

// RemoveFirst drops the first n items by advancing start—no data copy needed
func (rb *RollingBuffer) RemoveFirst(n int) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if n >= rb.size {
		rb.start, rb.size = 0, 0
		return
	}
	rb.start = (rb.start + n) % rb.capacity
	rb.size -= n
}

// Size reports number of items in buffer
func (rb *RollingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}
