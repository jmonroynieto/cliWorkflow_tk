package main

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"reflect"
	"strings"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
	"mvdan.cc/sh/v3/syntax"
)

var (
	Version  string
	Revision = ".0"
	CommitId string
)

var app = &cli.Command{
	Name:        "shfossils",
	Description: "uncovers all commands used in a shell script or bash history",
	Version:     fmt.Sprintf("%s (%s)", Version+Revision, CommitId),
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "sample",
			Aliases: []string{"s"},
			Value:   false,
			Usage:   "sample 20% of nodes from a shell script or bash history, show internal parser type",
		},
		&cli.BoolFlag{
			Name:    "breakdown",
			Aliases: []string{"b"},
			Value:   false,
			Usage:   "show breakdown of commands used in a shell script or bash history",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		if cmd.Bool("sample") {
			for _, arg := range cmd.Args().Slice() {
				sampleNodes(arg)
			}
			return nil
		} else if cmd.Bool("breakdown") {
			for _, arg := range cmd.Args().Slice() {
				tokenBreakdown(arg)
			}
			return nil
		}

		for _, arg := range cmd.Args().Slice() {
			searchFile(arg)
		}
		return nil
	},
}

func main() {
	err := app.Run(context.Background(), os.Args)
	errorutils.WarnOnFail(err)
}

func searchFile(path string) {
	data, _ := os.ReadFile(path)
	lines := strings.Split(string(data), "\n")
	linecount := len(lines)
	logrus.Info("total lines: ", linecount)
	var cmds = make(map[string]int, 1024)

	for _, strLine := range lines {

		parser := syntax.NewParser()
		file, _ := parser.Parse(bytes.NewReader([]byte(strLine)), "")

		syntax.Walk(file, func(node syntax.Node) bool {
			call, ok := node.(*syntax.CallExpr)

			if ok && len(call.Args) > 0 {
				if lit, ok := call.Args[0].Parts[0].(*syntax.Lit); ok {
					cmds[lit.Value]++
				} else if _, ok := call.Args[0].Parts[0].(*syntax.ParamExp); ok {
					//TODO:complete param expansion
				} else if logrus.GetLevel() >= logrus.DebugLevel {
					logrus.Error("unexpected non-literal in node at command position")
					fmt.Printf("\033[1;31m%s\033[0m\n", strLine)
					syntax.DebugPrint(os.Stdout, node)
				}
			}
			return true
		})
	}
	cmdslice := make([]string, 0, len(cmds))
	for cmd := range cmds {
		cmdslice = append(cmdslice, cmd)
	}
	sortByClean(cmdslice)
	for _, cmd := range cmdslice {
		if cmd == "" || cmd == " " {
			continue
		}
		fmt.Printf("%s\n", cmd)
	}
}

func sampleNodes(path string) {

	data, _ := os.ReadFile(path)
	parser := syntax.NewParser()
	file, _ := parser.Parse(bytes.NewReader(data), "")
	nodes := make(map[string]string, 10)

	syntax.Walk(file, func(node syntax.Node) bool {
		var buf bytes.Buffer
		if node != nil {
			printer := syntax.NewPrinter()
			_ = printer.Print(&buf, node)
		} else {
			return false
		}
		if rand.Float64() <= 0.20 || strings.HasPrefix(buf.String(), "mmpd") {
			key := buf.String()
			if len(key) > 120 {
				key = key[:120] + "..." // truncate long
			}
			nodes[key] = reflect.TypeOf(node).String()
		}
		return true
	})

	writeCSV(os.Stdout, nodes)
}

func tokenBreakdown(path string) {
	data, _ := os.ReadFile(path)
	lines := strings.Split(string(data), "\n")
	linecount := len(lines)
	logrus.Info("total lines: ", linecount)

	for line, strLine := range lines {
		if rand.Float32() <= 0.0005 {
			parser := syntax.NewParser()
			file, _ := parser.Parse(bytes.NewReader([]byte(strLine)), "")
			// logrus.Info("total statements: ", len(file.Stmts))
			for _, stmt := range file.Stmts {
				if stmt == nil {
					continue
				}
				printStatement(line, stmt)
				dumpNode(stmt)
			}
		}
	}
}
