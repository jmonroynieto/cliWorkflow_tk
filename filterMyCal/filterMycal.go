package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	// Define flags
	daysFlag := flag.String("d", "", "comma-separated list of days to retain")
	fileArg := flag.String("f", "", "CSV file to process")
	flag.Parse()
	// if flags are not provided, print usage and exit
	if *daysFlag == "" || *fileArg == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Parse daysFlag into a map for easy lookup
	daysToRetain := make(map[string]bool)
	if *daysFlag != "" {
		for _, segment := range strings.Split(*daysFlag, ",") {
			if strings.Contains(segment, "-") {
				// Handle day ranges
				rangeStart, _ := strconv.Atoi(strings.Split(segment, "-")[0])
				rangeEnd, _ := strconv.Atoi(strings.Split(segment, "-")[1])
				for i := rangeStart; i <= rangeEnd; i++ {
					daysToRetain[fmt.Sprintf("%02d", i)] = true
				}
			} else {
				// Handle individual days
				daysToRetain[segment] = true
			}
		}
	}
	// Open fileArg for reading
	file, err := os.Open(*fileArg)
	if err != nil {
		fmt.Printf("Error opening file: %v", err)
		os.Exit(1)
	}
	defer file.Close()

	// Read CSV from file
	reader := csv.NewReader(file)
	lines, err := reader.ReadAll()
	if err != nil {
		panic(err)
	}

	// Filter and retain specific days and header line
	var filteredLines [][]string
	for i, line := range lines {
		if i == 0 || daysToRetain[line[1][:2]] {
			filteredLines = append(filteredLines, line)
		}
	}

	// Open fileArg for writing
	file, err = os.Create(*fileArg)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Write filtered CSV to file
	writer := csv.NewWriter(file)
	for _, line := range filteredLines {
		err := writer.Write(line)
		if err != nil {
			panic(err)
		}
	}
	writer.Flush()

	fmt.Println("File processed successfully.")
}
