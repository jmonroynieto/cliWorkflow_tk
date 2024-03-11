package main

import (
	"fmt"
	"time"
)

//var dawnOJuan = time.Date(2018, 3, 1, 0, 0, 0, 0, time.UTC)

func parseMonthYear(dateStr string) (int, int, error) {
	// Define potential layouts for parsing
	layouts := []string{
		"Jan 2006", "January 2006",
		"2006 Jan", "2006 January",
		"Jan 06", "January 06",
		"06 Jan", "06 January",
		"Jan '06", "January '06",
		"'06 Jan", "'06 January",
		"06/01", "06-01", "6/1", "6-1", "2006 01",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, dateStr)
		if err == nil {
			return int(t.Month()), t.Year(), nil
		}
	}
	return 0, 0, fmt.Errorf("error parsing date: %s", dateStr)
}

func transform(y, m int) int {
	return ((-4 * y) + 8294) - ((m - 1) / 3)
}

func getToday() (int, int, error) {
	t := time.Now()
	return int(t.Month()), t.Year(), nil
}
