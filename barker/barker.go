package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/TGenNorth/errorutils"
	"github.com/go-mail/mail"
	"github.com/sirupsen/logrus"
)

func main() {
	//quick CLI
	fs := flag.NewFlagSet("barker", flag.ExitOnError)
	file := fs.String("file", "", "The directory to monitor.")
	recipient := fs.String("recipient", "", "The email recipient.")
	tmplt := fs.Bool("exp", false, "Export commands to set the SMTP_SENDER and SMTP_PASSWORD environment variables.")
	walltime := fs.Int("walltime", 0, "The time in hours to run the program for. The program will never wait indefinitely, min=1, max=168.")
	//Usage flag
	fs.Parse(os.Args[1:])
	//when template is specified, print the template and exit
	if *tmplt {
		fmt.Println("export SMTP_SENDER=\"\"")
		fmt.Println("export SMTP_PASSWORD=\"\"")
		os.Exit(0)
	}
	if *file == "" {
		errorutils.PanicOnFail(
			errorutils.NewReport("BARKER: Please specify the directory or file to monitor.", ""),
			errorutils.WithExitCode(1),
		)
	}
	if *recipient == "" {
		errorutils.PanicOnFail(
			errorutils.NewReport("BARKER: Please specify the email recipient.", ""),
			errorutils.WithExitCode(1),
		)
		return
	}

	logrus.Info("BARKER: remember that the security of your password is your responsibility. To avoid text saves of your password, please avoid writing the export statement in `.bashrc` or `.bash_profile`. Additionally, HISTIGNORE can be used to avoid saving the command to your shell's history. e.g. `HISTIGNORE='*SMTP_PASSWORD*`")

	if *walltime == 0 {
		logrus.Info("BARKER: No walltime specified, defaulting to 6 hours")
		*walltime = int(6)
	} else if *walltime < 1 || *walltime > 168 {
		errorutils.PanicOnFail(
			errorutils.NewReport(fmt.Sprintf("BARKER: Please specify a walltime between 1 and 168 hours. You specified: %d", *walltime), ""),
			errorutils.WithExitCode(1),
		)
		return
	}
	//start timer
	wt := time.NewTimer(time.Duration(*walltime) * time.Hour)

	//email setup
	sender := os.Getenv("SMTP_SENDER")
	password := os.Getenv("SMTP_PASSWORD")

	if sender == "" || password == "" {
		errorutils.WarnOnFail(
			errorutils.NewReport("BARKER: Please specify the sender and password by setting environment variables `SMTP_SENDER` and `SMTP_PASSWORD`\nRemember that app passwords for gmail are needed. See https://support.google.com/accounts/answer/185833?hl=en", ""),
			errorutils.WithExitCode(3),
		)
		os.Exit(3)
	}
	msg := mail.NewMessage()
	msg.SetHeader("From", sender)
	msg.SetHeader("To", *recipient)
	t := time.Now()
	stringTime := t.Format("2006-01-02 15:04:05")
	msg.SetHeader("Subject", fmt.Sprintf("Barker: change at %s", stringTime))
	msg.SetBody("text/plain", fmt.Sprintf("%s\nBarker: your file %s has been created", stringTime, *file))
	d := mail.NewDialer("smtp.gmail.com", 587, sender, password)

	//monitoring
	var err2 error = os.ErrNotExist
	var info os.FileInfo
	tick := time.NewTicker(5 * time.Second)
	var errCounter int //only warn 3 times for unexpected errors
LOOP:
	for {
		select {
		case <-wt.C:
			ranOutOfTime(*walltime)
		case <-tick.C:
			info, err2 = os.Stat(*file)
			if err2 == nil {
				break LOOP
			} else if os.IsNotExist(err2) {
				continue
			} else if err2 != nil && errCounter < 3 {
				errorutils.WarnOnFail(err2)
				errCounter++
				continue
			} else if errCounter == 3 {
				errorutils.PanicOnFail(err2, errorutils.WithExitCode(3))
			}

		}
	}

	// attach info to email
	temp, err := os.CreateTemp("", "barker.*.tmp")
	errorutils.PanicOnFail(err)
	_, err = temp.WriteString(fmt.Sprintf("Name: %s\nSize: %d\nMode: %s\nModTime: %s\nIsDir: %t\nSys: %v\n", info.Name(), info.Size(), info.Mode(), info.ModTime(), info.IsDir(), info.Sys()))
	errorutils.PanicOnFail(err)
	defer errorutils.NotifyClose(temp)
	msg.Attach(temp.Name())
	err = d.DialAndSend(msg)
	errorutils.PanicOnFail(err)
}

func ranOutOfTime(walltime int) {
	errorutils.PanicOnFail(fmt.Errorf("BARKER: the walltime timer has run out (time = %v), barker is shutting down", walltime), errorutils.WithExitCode(4))
}
