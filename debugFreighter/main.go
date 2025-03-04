// Package main implements a source code processor for Go files that identifies Logrus debug calls,
// extracts the context lines into a JSON file, removes debug lines from the source,
// and restores them later using extended context matching.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
)

// DebugEntry stores a debug call along with its context lines.
type DebugEntry struct {
	DebugLine      string   `json:"debug_line"`
	BeforeLine     string   `json:"before_line"`
	AfterLine      string   `json:"after_line"`
	ExtendedBefore []string `json:"extended_before"` // two lines preceding BeforeLine (if available)
	ExtendedAfter  []string `json:"extended_after"`  // two lines following AfterLine (if available)
}

// readLines reads the entire file into a slice of strings.
func readLines(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// writeLines writes the lines slice to the given filename.
func writeLines(filename string, lines []string) error {
	data := strings.Join(lines, "\n")
	return os.WriteFile(filename, []byte(data), 0644)
}

// extractDebugEntries scans the lines for Logrus debug calls.
// For each debug call found at index i, it stores:
// - The debug line itself.
// - The immediate previous line (BeforeLine) and immediate next line (AfterLine), if available.
// - Extended context: two lines before the BeforeLine and two lines after the AfterLine.
func extractDebugEntries(lines []string, contextAmount int) []DebugEntry {
	// Regular expression to match log.Debug( or log.Debugf(
	re := regexp.MustCompile(`\blog\.Debug(f?)\s*\(`)
	var entries []DebugEntry

	// Iterate over all lines.
	for i, line := range lines {
		if re.MatchString(line) {
			entry := DebugEntry{
				DebugLine: line,
			}
			// Get the immediate previous line.
			if i-1 >= 0 {
				entry.BeforeLine = lines[i-1]
			}
			// Get the immediate next line.
			if i+1 < len(lines) {
				entry.AfterLine = lines[i+1]
			}
			// Extended before: collect lines before the BeforeLine based on context amount.
			for j := 2; j < contextAmount && i-j >= 0; j++ {
				entry.ExtendedBefore = append([]string{lines[i-j]}, entry.ExtendedBefore...)
			}

			// Extended after: collect lines after the AfterLine based on context amount.
			for j := 2; j < contextAmount && i+j < len(lines); j++ {
				entry.ExtendedAfter = append(entry.ExtendedAfter, lines[i+j])
			}
			entries = append(entries, entry)
		}
	}
	return entries
}

// saveDebugEntriesJSON saves the debug entries to a JSON file.
func saveDebugEntriesJSON(filename string, entries []DebugEntry) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

// loadDebugEntriesJSON loads the debug entries from a JSON file.
func loadDebugEntriesJSON(filename string) ([]DebugEntry, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var entries []DebugEntry
	err = json.Unmarshal(data, &entries)
	return entries, err
}

// removeDebugLines returns a new slice of lines with any line that matches a debug call removed.
func removeDebugLines(lines []string) []string {
	re := regexp.MustCompile(`\blog\.Debug(f?)\s*\(`)
	var newLines []string
	for _, line := range lines {
		if !re.MatchString(line) {
			newLines = append(newLines, line)
		}
	}
	return newLines
}

// insertAt inserts a new line into the slice at the given index.
func insertAt(lines []string, index int, newLine string) []string {
	if index < 0 || index > len(lines) {
		return lines
	}
	result := make([]string, 0, len(lines)+1)
	result = append(result, lines[:index]...)
	result = append(result, newLine)
	result = append(result, lines[index:]...)
	return result
}

// findCandidates searches the current file lines for positions where the stored
// BeforeLine and AfterLine (with extended context) match.
// It returns the candidate indices where the debug line should be inserted (between before and after).
func findCandidates(lines []string, entry DebugEntry) []int {
	var candidates []int
	// We start from index 1 because we need a line before.
	for i := 1; i < len(lines); i++ {
		if lines[i-1] == entry.BeforeLine && lines[i] == entry.AfterLine {
			// Check extendedBefore: if available, ensure the preceding lines match.
			extBeforeMatch := true
			if len(entry.ExtendedBefore) > 0 {
				start := i - 1 - len(entry.ExtendedBefore)
				if start < 0 {
					extBeforeMatch = false
				} else {
					for j, extLine := range entry.ExtendedBefore {
						if lines[start+j] != extLine {
							extBeforeMatch = false
							break
						}
					}
				}
			}
			// Check extendedAfter: if available, ensure following lines match.
			extAfterMatch := true
			if len(entry.ExtendedAfter) > 0 {
				if i+len(entry.ExtendedAfter) >= len(lines) {
					extAfterMatch = false
				} else {
					for j, extLine := range entry.ExtendedAfter {
						if lines[i+1+j] != extLine {
							extAfterMatch = false
							break
						}
					}
				}
			}
			if extBeforeMatch && extAfterMatch {
				candidates = append(candidates, i)
			}
		}
	}
	return candidates
}

// restoreDebugEntries processes each DebugEntry and inserts its debug line into the current lines.
// If exactly one candidate match is found, it inserts the debug line as active code.
// If zero or multiple candidates are found (or if only one of the surrounding lines exists),
// it inserts the debug line as a commented-out line (prefixed with "// ").
func restoreDebugEntries(currentLines []string, entries []DebugEntry) []string {
	// Process each debug entry one by one.
	// To avoid index shifting, we process debug entries in reverse order of their candidate indices
	// within the file. (Note: This simplistic implementation does not handle overlapping candidate regions.)
	for _, entry := range entries {
		candidates := findCandidates(currentLines, entry)
		if len(candidates) == 1 {
			// Exactly one match: insert active debug line between the before and after lines.
			idx := candidates[0]
			// Insert the debug line between idx-1 (before) and idx (after)
			currentLines = insertAt(currentLines, idx, entry.DebugLine)
			fmt.Printf("Restored active debug line at index %d\n", idx)
		} else if len(candidates) > 1 {
			// Ambiguous match: insert the debug line as a comment at each candidate.
			// Process in descending order to avoid index shifting.
			sort.Sort(sort.Reverse(sort.IntSlice(candidates)))
			for _, idx := range candidates {
				commented := "// " + entry.DebugLine
				currentLines = insertAt(currentLines, idx, commented)
				fmt.Printf("Restored debug line as comment at ambiguous candidate index %d\n", idx)
			}
		} else {
			// No candidate with both surrounding lines found.
			// Try to find a candidate using only the before or only the after line.
			idxBefore := -1
			idxAfter := -1
			for i, line := range currentLines {
				if idxBefore == -1 && line == entry.BeforeLine {
					idxBefore = i
				}
				if idxAfter == -1 && line == entry.AfterLine {
					idxAfter = i
				}
			}
			commented := "// " + entry.DebugLine
			if idxBefore != -1 {
				currentLines = insertAt(currentLines, idxBefore+1, commented)
				fmt.Printf("Restored debug line as comment after before-line at index %d\n", idxBefore)
			} else if idxAfter != -1 {
				currentLines = insertAt(currentLines, idxAfter, commented)
				fmt.Printf("Restored debug line as comment before after-line at index %d\n", idxAfter)
			} else {
				// As a fallback, append the debug line as a comment at the end.
				currentLines = append(currentLines, commented)
				fmt.Printf("Restored debug line as comment at end of file\n")
			}
		}
	}
	return currentLines
}

func main() {
	// Define command-line flags.
	mode := flag.String("mode", "extract", "Mode: extract, remove, restore")
	inputFile := flag.String("input", "", "Input Go source file")
	outputFile := flag.String("output", "", "Output file: JSON file for extract, Go source file for remove/restore")
	jsonFile := flag.String("json", "debug_entries.json", "JSON file used for storing/restoring debug entries (for restore mode)")
	context := flag.Int("context", 1, "Number of context lines before and after debug line to capture (immediate before/after)")
	flag.Parse()

	if *inputFile == "" || *outputFile == "" {
		fmt.Println("Usage: -input=<source.go> -output=<file> -mode=<extract|remove|restore> [-json=<debug.json>]")
		os.Exit(1)
	}

	// Read the input source file.
	lines, err := readLines(*inputFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	switch *mode {
	case "extract":
		// Extract debug entries from the source file.
		entries := extractDebugEntries(lines, *context)
		// Save the extracted entries into a JSON file.
		if err := saveDebugEntriesJSON(*outputFile, entries); err != nil {
			log.Fatalf("Failed to save debug entries to JSON: %v", err)
		}
		fmt.Printf("Extraction complete. JSON saved to %s with %d entries.\n", *outputFile, len(entries))
	case "remove":
		// Remove debug lines from the source file.
		cleanLines := removeDebugLines(lines)
		if err := writeLines(*outputFile, cleanLines); err != nil {
			log.Fatalf("Failed to write output file: %v", err)
		}
		fmt.Printf("Removal complete. Clean file saved to %s.\n", *outputFile)
	case "restore":
		// Load previously saved debug entries from JSON.
		entries, err := loadDebugEntriesJSON(*jsonFile)
		if err != nil {
			log.Fatalf("Failed to load debug entries from JSON: %v", err)
		}
		// Restore debug lines into the current file based on context matching.
		restoredLines := restoreDebugEntries(lines, entries)
		// Write the updated file.
		if err := writeLines(*outputFile, restoredLines); err != nil {
			log.Fatalf("Failed to write restored file: %v", err)
		}
		fmt.Printf("Restoration complete. Restored file saved to %s.\n", *outputFile)
	default:
		fmt.Println("Unknown mode. Use extract, remove, or restore.")
	}
}
