package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func getIgnoreRegexes(filePath string) ([]*regexp.Regexp, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")

	var regexes []*regexp.Regexp
	for _, line := range lines {
		if line == "" {
			continue
		}
		re, err := regexp.Compile(line)
		if err != nil {
			return nil, fmt.Errorf("error compiling regex pattern '%s': %w", line, err)
		}
		regexes = append(regexes, re)
	}

	return regexes, nil
}

func IsIgnored(text string) bool {
	for _, re := range ignorable {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}
