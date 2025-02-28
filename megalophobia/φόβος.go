package main

import (
	"context"
	"fmt"
	"time"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

func phobia(ctx context.Context, cmd *cli.Command) error {
	err := SetupCapture()
	errorutils.ExitOnFail(err)
	go AsyncUpdateBuffer()
	var sentinel int
	strings := []string{
		"Program starting...",
		"This is the first line of output.",
		"Here's a second line — following right after.",
		"Line number three is now being printed.",
		"And yet another line — shifting the window.",
		"Fifth line arriving — demonstrating rolling effect.",
		"Accidentally spilled coffee on the PCR machine. Again.",
		"Is it just me or does the autoclave sound angrier today?",
		"Pretty sure I just pipetted air.  Into the cell culture. Oops.",
		"Myoglobin smells faintly of...regret?",
		"Experiment 'Unforeseen Protein Interactions' started.", // Lab Event starts - but just printed now
		"Note to self: Always double-check buffer pH. Cells now look sad.",
		"Where did I put the stock solution of GFP?  *Checks fridge, freezer, under desk*",
		"The gel ran...sideways? Is that even possible?",
		"Uh oh, the vortex mixer is vibrating its way off the bench.",
		"Sample 'Mystery Precipitate' analysis initiated.", // Lab Event starts - but just printed now
		"Pretty sure that wasn't supposed to bubble THAT much.",
		"Found the GFP. It was in my coat pocket. Classic lab life.",
		"Okay, who put the sticky notes IN the centrifuge?",
		"Maybe if I just squint, the contamination in the petri dish will disappear.",
		"Wait, did I add protease inhibitors to *that* sample?",
		"The good news: I finished my experiment. The bad news: I used water instead of enzyme.",
		"Is it normal for the spectrophotometer to make beeping noises like that?",
		"Just heard someone yell 'FIRE!' Turns out it was just toast.",
		"Pretty sure the incubator is judging my experimental design.",
		"Final line to show the three-line effect clearly.",
	}

	for _, s := range strings {
		fmt.Println(s)
		sentinel += 1
		time.Sleep(300 * time.Millisecond)
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.Debug(fmt.Sprintf("Sentinel: %d", sentinel))
			lastDisplayLines += 1
		}
	}
	close(stopChan)
	wg.Wait()
	return err
}
