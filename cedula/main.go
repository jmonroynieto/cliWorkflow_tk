package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
)

var (
	Version  string
	Revision = ".0"
	CommitId string
)

type notepad struct {
	funcs      map[string]struct{}
	varsConsts map[string]struct{}
	types      map[string]struct{}
}

func main() {
	packagePath := validArguments(os.Args)
	fset := token.NewFileSet()
	packages, err := parser.ParseDir(fset, packagePath, nil, 0)
	if err != nil {
		fmt.Printf("Error parsing directory: %v\n", err)
		return
	}
	notes := notepad{
		make(map[string]struct{}),
		make(map[string]struct{}),
		make(map[string]struct{}),
	}

	for _, pkg := range packages {
		for _, file := range pkg.Files {
			//iterator
			ast.Inspect(file, func(n ast.Node) bool {
				typeSorter(n, &notes)
				return true
			})
		}
	}

	funcNames, varConstNames, typeNames := notes.Sort()

	prettyPrint("Functions", funcNames)
	prettyPrint("Variables/Constants", varConstNames)
	prettyPrint("Types", typeNames)
}

func typeSorter(n ast.Node, notes *notepad) {
	switch x := n.(type) {
	case *ast.GenDecl: // Covers var, const, type declarations
		for _, spec := range x.Specs {
			if vs, ok := spec.(*ast.ValueSpec); ok {
				for _, id := range vs.Names {
					if id.Name != "_" {
						notes.varsConsts[id.Name] = struct{}{}
					}
				}
			} else if ts, ok := spec.(*ast.TypeSpec); ok {
				if ts.Name.Name != "_" {
					notes.types[ts.Name.Name] = struct{}{}
				}
			}
		}
	case *ast.FuncDecl: // Covers function declarations
		if x.Name.Name != "_" {
			notes.funcs[x.Name.Name] = struct{}{}
		}
	}
}

func (n *notepad) Sort() ([]string, []string, []string) {
	funcNames := make([]string, 0, len(n.funcs))
	for name := range n.funcs {
		funcNames = append(funcNames, name)
	}
	sort.Strings(funcNames)

	varConstNames := make([]string, 0, len(n.varsConsts))
	for name := range n.varsConsts {
		varConstNames = append(varConstNames, name)
	}
	sort.Strings(varConstNames)

	typeNames := make([]string, 0, len(n.types))
	for name := range n.types {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)

	return funcNames, varConstNames, typeNames
}
func validArguments(args []string) string {
	if len(args) < 2 || len(args) > 2 {
		fmt.Printf("\x1b[31mError\x1b[0m: Invalid number of arguments\n")
		fmt.Println("Usage: cedula <path_to_package_directory>")
		os.Exit(1)
	}
	switch args[1] {
	case "-h", "--help":
		fmt.Println("Usage: cedula <path_to_package_directory>")
		os.Exit(0)
	case "-v", "--version":
		fmt.Printf("cedula %s%s (%s)\n", Version, Revision, CommitId)
		os.Exit(0)
	}

	packagePath := args[1]
	if fi, err := os.Stat(packagePath); os.IsNotExist(err) {
		fmt.Printf("\x1b[31mError\x1b[0m: Directory %s does not exist\n", packagePath)
		os.Exit(1)
	} else if !fi.IsDir() {
		fmt.Printf("Error: %s is not a directory\n", packagePath)
		os.Exit(1)
	} else if err != nil {
		fmt.Printf("\x1b[31mError\x1b[0m: %v\n", err)
	}
	return packagePath

}
func prettyPrint(title string, names []string) {
	if len(names) > 0 {
		fmt.Printf("\t\x1b[33m[%s]\x1b[0m\n", title)
		for _, name := range names {
			fmt.Println(name)
		}
	}
}
