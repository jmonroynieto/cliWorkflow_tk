package main

import (
	"fmt"
	"math/rand/v2"

	"github.com/urfave/cli/v2"
)

var (
	n         int // number of characters to generate
	noNewline bool
)

func generateID(cCtx *cli.Context) error {

	var b = make([]byte, 0, n)
	var formatTemplate string
	for len(b) < n {
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
		{48, 57},
		{65, 90},
		{97, 122},
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
