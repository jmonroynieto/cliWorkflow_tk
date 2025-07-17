package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/go-mail/mail"
	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

var (
	Version string
	Revision = ".0"
	CommitId string
)

func main() {
	sort.Sort(cli.FlagsByName(appFlags))
	app := &cli.Command{
		Name:     "barker",
		Usage:    "usage",
		Flags:    appFlags,
		Commands: appCmds,
		Version:  fmt.Sprintf("%s (%s)", Version+Revision, CommitId),
	}

	app.Run(context.Background(), os.Args)
}

var appFlags []cli.Flag = []cli.Flag{
	&cli.BoolFlag{
		Name:    "debug",
		Aliases: []string{"d"},
		Usage:   "activates debugging messages",
		Action: func(ctx context.Context, cmd *cli.Command, shouldDebug bool) error {
			if shouldDebug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil

		},
	},
	&cli.StringFlag{
		Name:    "dir",
		Aliases: []string{"file", "a"},
		Usage:   "The `DIR` to monitor",
	},
	&cli.StringFlag{
		Name:    "recipient",
		Aliases: []string{"r"},
		Usage:   "email `ADDRESS` to report to",
	},
	&cli.BoolFlag{
		Name:  "exp",
		Usage: "Export commands to set the SMTP_SENDER and SMTP_PASSWORD environment variables",
	},
	&cli.IntFlag{
		Name:        "walltime",
		Value:       24,
		Usage:       "timer to stop. BARKER never runs forever",
		DefaultText: "1 day",
	},
}

var appCmds []*cli.Command = []*cli.Command{
	{
		Name:   "name",
		Usage:  "Usage",
		Action: action1,
	},
}

func action1(ctx context.Context, cmd *cli.Command) error {
	file := cmd.String("dir")
	recipient := cmd.String("recipient")
	walltime := cmd.Int("walltime")
	//when template is specified, print the template and exit
	if cmd.Bool("exp") {
		fmt.Println("export SMTP_SENDER=\"\"")
		fmt.Println("export SMTP_PASSWORD=\"\"")
		os.Exit(0)
	}
	if file == "" {
		errorutils.ExitOnFail(
			errorutils.NewReport("BARKER: Please specify the directory or file to monitor.", ""),
			errorutils.WithExitCode(1),
		)
	}
	if recipient == "" {
		errorutils.ExitOnFail(
			errorutils.NewReport("BARKER: Please specify the email recipient.", ""),
			errorutils.WithExitCode(1),
		)
	}

	logrus.Info("BARKER: remember that the security of your password is your responsibility. To avoid text saves of your password, please avoid writing the export statement in `.bashrc` or `.bash_profile`. Additionally, HISTIGNORE can be used to avoid saving the command to your shell's history. e.g. `HISTIGNORE='*SMTP_PASSWORD*`")

	if walltime < 1 || walltime > 168 {
		errorutils.ExitOnFail(
			errorutils.NewReport(fmt.Sprintf("BARKER: Please specify a walltime between 1 and 168 hours. You specified: %d", walltime), ""),
			errorutils.WithExitCode(1),
		)
	}
	//start timer
	wt := time.NewTimer(time.Duration(walltime) * time.Hour)

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
	msg.SetHeader("To", recipient)
	t := time.Now()
	stringTime := t.Format("2006-01-02 15:04:05")
	msg.SetHeader("Subject", fmt.Sprintf("Barker: change at %s", stringTime))
	msg.SetBody("text/plain", fmt.Sprintf("%s\nBarker: your file %s has been created", stringTime, file))
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
			ranOutOfTime(int(walltime))
		case <-tick.C:
			info, err2 = os.Stat(file)
			if err2 == nil {
				break LOOP
			} else if os.IsNotExist(err2) {
				continue
			} else if errCounter < 3 {
				errorutils.WarnOnFail(err2)
				errCounter++
				continue
			} else if errCounter == 3 {
				errorutils.ExitOnFail(err2, errorutils.WithExitCode(3))
			}

		}
	}

	// attach info to email
	temp, err := os.CreateTemp("", "barker.*.tmp")
	errorutils.ExitOnFail(err)
	_, err = temp.WriteString(fmt.Sprintf("Name: %s\nSize: %d\nMode: %s\nModTime: %s\nIsDir: %t\nSys: %v\n", info.Name(), info.Size(), info.Mode(), info.ModTime(), info.IsDir(), info.Sys()))
	errorutils.ExitOnFail(err)
	defer errorutils.NotifyClose(temp)
	msg.Attach(temp.Name())
	err = d.DialAndSend(msg)
	errorutils.ExitOnFail(err)
	return nil
}

func ranOutOfTime(walltime int) {
	errorutils.ExitOnFail(fmt.Errorf("BARKER: the walltime timer has run out (time = %v), barker is shutting down", walltime), errorutils.WithExitCode(4))
}
