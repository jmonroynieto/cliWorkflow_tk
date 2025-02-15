package main

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/urfave/cli/v3"
)

var (
	aAmount int64
	bStart  int64
	cEnd    int64
)

func generateJitter(c context.Context, cmd *cli.Command) error {
	numbers := generateNumbers(aAmount, float64(bStart), float64(cEnd))

	// Print the generated numbers
	for _, num := range numbers {
		fmt.Printf("%.3f\n", num)
	}

	return nil
}

var jitterFlags []cli.Flag = []cli.Flag{
	&cli.IntFlag{
		Name:        "amount",
		Aliases:     []string{"a"},
		Usage:       "amount of numbers to generate",
		Destination: &aAmount,
		Required:    false,
		Value:       10,
	},
	&cli.IntFlag{
		Name:        "start",
		Aliases:     []string{"s"},
		Usage:       "start of the range",
		Destination: &bStart,
		Required:    false,
		Value:       0,
	},
	&cli.IntFlag{
		Name:        "end",
		Aliases:     []string{"e"},
		Usage:       "end of the range",
		Destination: &cEnd,
		Required:    false,
		Value:       1,
	},
}

func generateNumbers(amount int64, start, end float64) []float64 {
	rangeSize := float64(end - start)
	numbers := make([]float64, 0, amount)
	first := start + rand.Float64()*rangeSize
	numbers = append(numbers, first)

	for len(numbers) < int(amount) {
		var next float64
		for {
			next = start + rand.Float64()*rangeSize
			if isValidDifference(numbers, next, rangeSize) {
				break
			}
		}
		numbers = append(numbers, next)
	}

	return numbers
}

func isValidDifference(numbers []float64, next, rangeSize float64) bool {
	minDiff := 0.15 * rangeSize
	minDiff2 := 0.05 * rangeSize
	last := numbers[len(numbers)-1]
	secondLast := next - minDiff2*2.0
	if len(numbers) > 2 {
		secondLast = numbers[len(numbers)-2]
	}
	return abs(last-next) >= minDiff && abs(secondLast-next) >= minDiff2
}

func abs(a float64) float64 {
	if a < 0 {
		return -a
	}
	return a
}
