package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	colorOrange = "\033[38;5;214m"
	colorReset  = "\033[0m"
	helpText    = `kompti requires month and year in any of a variety of formats. When no arguments are provided, current time is printed`
)

var (
	Version  string
	Revision = ".0"
	CommitId string
)

func main() {
	var args = os.Args
	regex := regexp.MustCompile(`^\d{1,4}$`)
	if len(args) == 1 {
		m, y, _ := getToday()
		q := transformInto(y, m)
		fmt.Printf("%d\n", q)
	} else if args[1] == "help" || args[1] == "-h" || args[1] == "--help" {
		fmt.Printf("%s (%s)\n", Version+Revision, CommitId)
		fmt.Println(helpText)
	} else if len(args) == 2 && regex.MatchString(args[1]) {
		q, _ := strconv.Atoi(args[1])
		y, m := transformBack(q)
		fmt.Printf("%d %d\n", m, y)
	} else {
		dateStr := strings.Join(args[1:], ` `)
		m, y, e := parseMonthYear(dateStr)
		if e != nil {
			fmt.Printf("ERROR: %sCan't parse provided date: %q%s\n", colorOrange, dateStr, colorReset)
			os.Exit(3)
		}
		q := transformInto(y, m)
		fmt.Println(q)
	}
}
