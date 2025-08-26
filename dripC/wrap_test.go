package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// LLM generated test
func TestWrapping(t *testing.T) {
	// Example list of items
	items := []string{
		"Home", "About", "Services", "Portfolio", "Contact",
		"Blog", "News", "Events", "Gallery", "Team",
		"Documentation", "API", "Support", "Login", "Register",
	}

	// Use default config
	config := DefaultButtonConfig()

	// Get terminal width
	termWidth := GetTerminalWidth()
	fmt.Printf("Terminal width: %d\n", termWidth)

	// Wrap items to lines
	lines := portionItemsToLines(items, config, termWidth)

	// Display the result
	fmt.Printf("\nWrapped into %d lines:\n", len(lines))
	for i, line := range lines {
		fmt.Printf("Line %d (%d items): %s\n",
			i+1, len(line), strings.Join(line, " | "))

		// Show calculated width for this line
		lineWidth := 0
		for j, item := range line {
			itemWidth := config.CalculateItemWidth(item)
			lineWidth += itemWidth
			if j > 0 {
				lineWidth += config.ItemMargin
			}
		}
		fmt.Printf("  -> Total width: %d\n", lineWidth)
	}

	// Example with custom config
	fmt.Println("\n--- With custom config (more padding) ---")
	customConfig := ButtonConfig{
		HorizontalPadding: 4, // 2 spaces on each side
		ItemMargin:        2, // 2 spaces between items
		BorderWidth:       1,
		MinWidth:          8, // minimum 8 characters
	}

	customLines := portionItemsToLines(items, customConfig, termWidth)
	fmt.Printf("Custom config wrapped into %d lines:\n", len(customLines))
	for i, line := range customLines {
		fmt.Printf("Line %d: %s\n", i+1, strings.Join(line, " | "))
	}
}

func TestBuf(t *testing.T) {
	rb := NewRollingBuffer(6)
	for i := range 6 {
		rb.Add(fmt.Sprint(i))
	}
	fmt.Println(rb.GetAll()) // [1 2 3 4 5 6]
	rb.RemoveFirst(4)
	if !reflect.DeepEqual(rb.GetAll(), []string{"5", "6"}) {
		t.Errorf("expected [5 6], got %v", rb.GetAll())
	}
}

func TestCardPrint(t *testing.T) {
	f, _:= os.Open("./test.txt")
	cardPrint(f)
	fmt.Print("\033[0m")
}