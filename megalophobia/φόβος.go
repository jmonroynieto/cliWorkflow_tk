package main

import (
	"context"
	"fmt"
	"time"

	"github.com/pydpll/errorutils"
	"github.com/urfave/cli/v3"
)

func phobia(ctx context.Context, cmd *cli.Command) error {
	err := SetupCapture()
	errorutils.ExitOnFail(err)
	defer TeardownCapture() // returns stdout to original value and clears the display window
	fmt.Println("Program starting...")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("This is the first line of output.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Here's a second line — following right after.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Line number three is now being printed.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("And yet another line — shifting the window.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Fifth line arriving — demonstrating rolling effect.")
	time.Sleep(300 * time.Millisecond)
	// Funny lab mishap messages as fmt.Println statements
	fmt.Println("Accidentally spilled coffee on the PCR machine. Again.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Is it just me or does the autoclave sound angrier today?")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Pretty sure I just pipetted air.  Into the cell culture. Oops.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Myoglobin smells faintly of...regret?")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Experiment 'Unforeseen Protein Interactions' started.") // Lab Event starts - but just printed now
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Note to self: Always double-check buffer pH. Cells now look sad.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Where did I put the stock solution of GFP?  *Checks fridge, freezer, under desk*")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("The gel ran...sideways? Is that even possible?")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Uh oh, the vortex mixer is vibrating its way off the bench.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Sample 'Mystery Precipitate' analysis initiated.") // Lab Event starts - but just printed now
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Pretty sure that wasn't supposed to bubble THAT much.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Found the GFP. It was in my coat pocket. Classic lab life.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Okay, who put the sticky notes IN the centrifuge?")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Maybe if I just squint, the contamination in the petri dish will disappear.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("The good news: I finished my experiment. The bad news: I used water instead of enzyme.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Is it normal for the spectrophotometer to make beeping noises like that?")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Just heard someone yell 'FIRE!' Turns out it was just toast.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Wait, did I add protease inhibitors to *that* sample?")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Pretty sure the incubator is judging my experimental design.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Final line to show the three-line effect clearly.")
	time.Sleep(300 * time.Millisecond)
	return err
}
