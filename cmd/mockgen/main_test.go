package main_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

var env []string

func TestMockgen(t *testing.T) {
	ctx, eng := context.Background(), script.NewEngine()
	scripttest.Test(t, ctx, eng, env, "testdata/*.txt")
}

func TestMain(m *testing.M) {
	// Temporary directory for mockgen binary.
	bindir, err := os.MkdirTemp("", "mockgen")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(bindir)

	// The Module Under Test (MUT) directory.
	mutdir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mutdir = filepath.Join(mutdir, "..", "..")

	// When running with -test.gocoverdir, forward this setting to each
	// test script.
	fset := flag.NewFlagSet("mockgen", flag.ContinueOnError)
	fset.SetOutput(&bytes.Buffer{}) // ignore errors
	covdir := fset.String("test.gocoverdir", "", "write coverage intermediate files to this directory")
	err = fset.Parse(os.Args[1:])
	for err != nil {
		err = fset.Parse(fset.Args())
	}

	env = []string{
		"PATH=" + strings.Join([]string{bindir, os.Getenv("PATH")}, ":"),
		"HOME=" + os.Getenv("HOME"),
		"MUT=" + mutdir,
		"BINDIR=" + bindir,
	}
	if *covdir != "" {
		env = append(env, "GOCOVERDIR="+*covdir)
	}

	run(env, "testdata/setup.sh")
	m.Run()
	if *covdir != "" {
		run(env, "testdata/report.sh")
	}
}

func run(env []string, file string) {
	cmd := exec.Command("bash", file)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
