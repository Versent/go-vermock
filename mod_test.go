package vermock_test

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

func TestModules(t *testing.T) {
	ctx, eng := context.Background(), script.NewEngine()
	scripttest.Test(t, ctx, eng, env, "testdata/*.txt")
}

func TestMain(m *testing.M) {
	// Setup environment for scripttest.
	env = []string{
		"HOME=" + os.Getenv("HOME"),
	}

	// The Module Under Test (MUT) directory.
	mutdir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	env = append(env, "MUT="+mutdir)

	// When running with -test.gocoverdir, forward this setting to each
	// test script.
	fset := flag.NewFlagSet("vermockgen", flag.ContinueOnError)
	fset.SetOutput(&bytes.Buffer{}) // ignore errors
	covdir := fset.String("test.gocoverdir", "", "write coverage intermediate files to this directory")
	err = fset.Parse(os.Args[1:])
	for err != nil {
		err = fset.Parse(fset.Args())
	}

	if *covdir != "" {
		env = append(env, "GOCOVERDIR="+*covdir)
	}

	// Temporary directory for vermockgen binary.
	bindir, err := os.MkdirTemp("", "vermockgen")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(bindir)
	args := []string{"build"}
	if *covdir != "" {
		args = append(args, "-cover")
	}
	args = append(args, "-o", filepath.Join(bindir, "vermockgen"), "./cmd/vermockgen")
	cmd := exec.Command("go", args...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	env = append(env, "PATH="+strings.Join([]string{bindir, os.Getenv("PATH")}, ":"))

	m.Run()
}
