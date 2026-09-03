package main

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestReadLines(t *testing.T) {
	bashh, err := os.OpenFile("/home/pollo/.bash_history", os.O_RDONLY, 0o644)
	if err != nil {
		logrus.Warn(err)
		t.Fail()
	}

	defer bashh.Close()
	idx := indexLines(bashh)
	logrus.Info("memory usage of index:", 4*len(idx)/1024, "kb")

	bytes, err := idx.readlines(bashh, 21612, 21614)
	if err != nil {
		logrus.Warn(err)
		t.Fail()
	}

	t.Logf("%s", bytes)
}
