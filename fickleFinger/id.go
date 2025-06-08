package main

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/urfave/cli/v3"
)

var (
	n         int64 // number of characters to generate
	noNewline bool
)

func GenerateID(cCtx context.Context, cmd *cli.Command) error {

	var b = make([]byte, 0, n)
	var formatTemplate string
	for len(b) < int(n) {
		num := rand.N(122)
		if isInRange(num) {
			b = append(b, byte(num))
		}
	}
	if noNewline {
		formatTemplate = "%s"
	} else {
		formatTemplate = "%s\n"
	}
	fmt.Printf(formatTemplate, string(b))
	return nil

}
func isInRange(n int) bool {
	ranges := [][2]int{
		{48, 57}, // 0-9
		{65, 90}, // A-Z
		{97, 122}, // a-z
	}

	for _, r := range ranges {
		if n >= r[0] && n <= r[1] {
			return true
		}
	}
	return false
}

var idFlags []cli.Flag = []cli.Flag{
	&cli.IntFlag{
		Name:        "length",
		Aliases:     []string{"l"},
		Usage:       "length of the id",
		Destination: &n,
		Required:    false,
		Value:       11,
	},
	&cli.BoolFlag{
		Name:        "no_newline",
		Aliases:     []string{"n"},
		Usage:       "no new line at the end of the id",
		Destination: &noNewline,
	},
}
