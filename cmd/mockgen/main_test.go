package main_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

func TestMain(t *testing.T) {
	mutdir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	mutdir = filepath.Join(mutdir, "..", "..")
	scripttest.Test(
		t,
		context.Background(),
		script.NewEngine(),
		[]string{
			"PATH=" + os.Getenv("PATH"),
			"HOME=" + os.Getenv("HOME"),
			"MUT=" + mutdir,
		},
		"testdata/*.txt",
	)
}
