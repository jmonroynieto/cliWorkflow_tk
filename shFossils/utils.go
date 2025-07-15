package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"maps"
	"regexp"
	"sort"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

func writeCSV(w io.Writer, nodes map[string]string) {
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"NodeSnippet", "NodeType"})

	// Optional: deterministic output

	for k := range maps.Keys(nodes) {
		_ = writer.Write([]string{k, nodes[k]})
	}
	writer.Flush()
}

func printStatement(line int, stmt *syntax.Stmt) {
	var buf bytes.Buffer
	syntax.NewPrinter().Print(&buf, stmt)
	// bold blue for statement lines
	fmt.Printf("\033[1;34m%d:\033[0m \033[36m%s\033[0m\n", line, buf.String())
}
func printNode(node syntax.Node, depth int) {
	//syntax.DebugPrint(os.Stdout, node)
	// print this node
	indent := strings.Repeat("\t", depth)
	var buf bytes.Buffer
	syntax.NewPrinter().Print(&buf, node)
	fmt.Printf("%s└─ %T — %q\n", indent, node, buf.String())

}

func dumpNode(root syntax.Node) {
	var depth int
	syntax.Walk(root, func(n syntax.Node) bool {
		if n == nil {
			depth--
			return false
		}
		printNode(n, depth)
		depth++
		return true // recurse into children
	})
}

func cleanPath(s string) []byte {
	// Define the regular expression pattern to match unwanted prefixes
	pattern := regexp.MustCompile(`^(\.\.\/|\.\/|\.|~/|\\|{)`)

	for {
		// Find the longest prefix match
		prefix := pattern.FindString(s)
		if prefix == "" {
			break // No more prefixes to remove
		}
		// Remove the matched prefix
		s = s[len(prefix):]
	}

	// Handle cases where the entire string was one of the unwanted patterns
	if s == "." || s == ".." || s == "./" || s == "../" {
		return []byte("")
	}

	return []byte(strings.ToLower(s))
}

type item struct {
	orig string
	key  []byte
}

func sortByClean(cmds []string) {
	arr := make([]item, len(cmds))
	for i, s := range cmds {
		arr[i].orig = s
		arr[i].key = cleanPath(s) // sanitize into []byte
	}
	sort.Slice(arr, func(i, j int) bool {
		return bytes.Compare(arr[i].key, arr[j].key) < 0
	})
	for i := range cmds {
		cmds[i] = arr[i].orig
	}
}
