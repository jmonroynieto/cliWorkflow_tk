package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pydpll/errorutils"
)

var (
	Version  = "1.1.1"
	CommitId string
)

func main() {
	// Define flags
	daysFlag := flag.String("d", "", "comma-separated list of days to retain")
	fileArg := flag.String("f", "", "CSV file to process")
	versionPrint := flag.Bool("v", false, "print version of the tool")
	flag.Parse()

	if *daysFlag == "" || *fileArg == "" {
		flag.Usage()
		os.Exit(1)
	} else if *versionPrint {
		fmt.Printf("%s - %s", Version, CommitId)
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
	errorutils.ExitOnFail(err)

	// Filter and retain specific days and header line
	var filteredLines [][]string
	for i, line := range lines {
		if i == 0 || daysToRetain[line[1][:2]] {
			filteredLines = append(filteredLines, line)
		}
	}

	// Open fileArg for writing
	file, err = os.Create(*fileArg)
	errorutils.ExitOnFail(err)
	defer file.Close()

	// Write filtered CSV to file
	writer := csv.NewWriter(file)
	for _, line := range filteredLines {
		errorutils.ExitOnFail(writer.Write(line))
	}
	writer.Flush()

	fmt.Println("File processed successfully.")
}
