//calshow works with csv import files for gCal
package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func main() {
	//handle args
	if len(os.Args) < 2 {
		fmt.Println("Please provide the CSV file name as an argument")
		return
	}
	csvFileName := os.Args[1]
	csvFile, err := os.Open(csvFileName)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer csvFile.Close()

	
	// Create a map to store the events
	events := make(map[string][]string)
	reader := csv.NewReader(csvFile)
	reader.FieldsPerRecord = -1 // Allow variable number of fields per record.
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
			return
		}

		events[line[0]] = append(events[line[0]], line[1:]...)
	}
fmt.Println(events)

	
	// Print the week calendar view
	for day := 1; day <= 7; day++ {
		dayOfWeek := time.Now().AddDate(0, 0, day-1).Weekday()
		fmt.Println("###", time.Weekday(dayOfWeek).String(), "###")
		for _, event := range events {
			// Check if the event occurs on the current day.
			if event[1] == time.Weekday(dayOfWeek).String() {
				// Get the start and end times for the event.
				start, end := event[2], event[3]
				startStr := strings.Replace(start, ":", " ", 1)
				endStr := strings.Replace(end, ":", " ", 1)
				// Print the event.
				fmt.Printf("%s %s - %s\n", event[0], startStr, endStr)
			}
		}
	}
}

