package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"maps"
	"os"
	"reflect"
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
	fmt.Printf("\033[1;34m%d:\033[0m %s\n", line, buf.String())
}

func dumpNode(node syntax.Node, depth int) {
	if node == nil {
		return
	}
	syntax.DebugPrint(os.Stdout, node)
	// print this node
	indent := strings.Repeat("\t", depth)
	var buf bytes.Buffer
	syntax.NewPrinter().Print(&buf, node)
	fmt.Printf("%s└─ %T — %q\n", indent, node, buf.String())

	// walk children (return true to descend)
	syntax.Walk(node, func(child syntax.Node) bool {
		if child == nil {
			return false
		}
		if child != node {
			dumpNode(child, depth+1)
		}
		return true // allow descent into each child
	})
}

func dumpNode2(node syntax.Node, depth int) {
	if node == nil {
		return
	}

	// Print this node
	indent := strings.Repeat("\t", depth)
	var buf bytes.Buffer
	syntax.NewPrinter().Print(&buf, node)
	fmt.Printf("%s└─ %T — %q\n", indent, node, buf.String())

	// Get immediate children using reflection
	kids := getImmediateChildren(node)

	// Recurse on each immediate child
	for _, child := range kids {
		if child != nil {
			dumpNode2(child, depth+1)
		}
	}
}

func getImmediateChildren(node syntax.Node) []syntax.Node {
	if node == nil {
		return nil
	}

	var kids []syntax.Node
	seen := make(map[syntax.Node]bool)

	// Use reflection to inspect the node's fields
	val := reflect.ValueOf(node)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	// If it's not a struct, it has no children
	if val.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Handle pointer to Node
		if field.Kind() == reflect.Ptr {
			if node, ok := field.Interface().(syntax.Node); ok {
				if node != nil && !seen[node] {
					kids = append(kids, node)
					seen[node] = true
				}
			}
			continue
		}

		// Handle slice of Nodes
		if field.Kind() == reflect.Slice {
			// Check if the slice elements are Nodes
			if field.Type().Elem().Implements(reflect.TypeOf((*syntax.Node)(nil)).Elem()) {
				for j := 0; j < field.Len(); j++ {
					item := field.Index(j).Interface().(syntax.Node)
					if item != nil && !seen[item] {
						kids = append(kids, item)
						seen[item] = true
					}
				}
			}
			continue
		}

		// Handle direct Node value (not pointer)
		if node, ok := field.Interface().(syntax.Node); ok {
			if node != nil && !seen[node] {
				kids = append(kids, node)
				seen[node] = true
			}
			continue
		}

		// Handle struct fields that might themselves contain nodes
		if field.Kind() == reflect.Struct {
			if node, ok := field.Addr().Interface().(syntax.Node); ok {
				if node != nil && !seen[node] {
					kids = append(kids, node)
					seen[node] = true
				}
			}
		}
	}

	return kids
}
