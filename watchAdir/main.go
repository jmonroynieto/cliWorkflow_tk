package main

import (
	"context"
	"os"
	"time"

	"github.com/pydpll/errorutils"
)
var (
	Version    = "1.2.0"
	CommitId   string
	requestedTime time.Duration
)

func main() {
	deferErr := app.Run(context.Background(), os.Args)
	errorutils.WarnOnFail(deferErr, errorutils.WithMsg("app failed execution"))
}
