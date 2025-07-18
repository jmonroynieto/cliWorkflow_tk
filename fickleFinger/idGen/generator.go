package idGen

import (
	"fmt"
	"math/rand/v2"
)

func New(n int64, noNewline bool) string {
	b := make([]byte, 0, n)
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
	return fmt.Sprintf(formatTemplate, string(b))
}

func isInRange(n int) bool {
	ranges := [][2]int{
		{48, 57},  // 0-9
		{65, 90},  // A-Z
		{97, 122}, // a-z
	}

	for _, r := range ranges {
		if n >= r[0] && n <= r[1] {
			return true
		}
	}
	return false
}
