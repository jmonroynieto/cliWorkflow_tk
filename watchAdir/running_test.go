package main

import (
	"context"
	"os"
	"testing"

	"github.com/pydpll/errorutils"
)

func TestRunning(t *testing.T) {
	os.Chdir("/home/pollo/repobay/spuri")
	deferErr := app.Run(context.Background(), []string{"watchAdir", "-d", "2"})
	errorutils.WarnOnFail(deferErr, errorutils.WithMsg("app failed execution"))

}
