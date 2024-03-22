package vermock_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

var env []string
var covdir *string

func TestModules(t *testing.T) {
	ctx, eng := context.Background(), script.NewEngine()
	eng.Cmds["go-test"] = GoTestCommand(*covdir)
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
	covdir = fset.String("test.gocoverdir", "", "write coverage intermediate files to this directory")
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

type goTestCmd struct {
	covdir string
}

func GoTestCommand(covdir string) *goTestCmd {
	return &goTestCmd{covdir: covdir}
}

func (*goTestCmd) Usage() *script.CmdUsage {
	return &script.CmdUsage{
		Summary: "wrapper for the go test command",
		Args:    "go-test [build/test flags] [packages] [build/test flags & test binary flags]",
		Detail: []string{
			`The standard go test command output includes data about elapsed time and code `,
			`coverage. This can be a hinderance when comparing the output of two test runs. `,
			`This wrapper will strip out all the non-reproducible data from the output. It `,
			`also automatically sets the -coverpkg and -test.gocoverdir flags if the `,
			`GOCOVERDIR environment variable is set.`,
		},
	}
}

func (cmd *goTestCmd) Run(s *script.State, args ...string) (script.WaitFunc, error) {
	var buf, stderr bytes.Buffer
	goargs := []string{"test", "-json"}
	if cmd.covdir != "" {
		goargs = append(
			goargs,
			"-coverpkg=github.com/Versent/go-vermock/...",
			"-test.gocoverdir="+cmd.covdir,
		)
	}
	goargs = append(goargs, args...)
	gocmd := exec.CommandContext(s.Context(), "go", goargs...)
	gocmd.Dir = s.Getwd()
	gocmd.Env = s.Environ()
	gocmd.Stdout = &buf
	gocmd.Stderr = &stderr
	err := gocmd.Start()

	return func(s *script.State) (_, _ string, err error) {
		err = gocmd.Wait()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			err = nil
		}
		if err != nil {
			println(stderr.String(), err.Error())
			return "", stderr.String(), err
		}

		var stdout strings.Builder
		decoder := json.NewDecoder(&buf)

		for decoder.More() {
			var event struct {
				Time    time.Time
				Action  string
				Package string
				Test    string
				Elapsed float64
				Output  string
			}
			if err := decoder.Decode(&event); err != nil {
				return "", stderr.String(), err
			}
			if event.Test == "" {
				continue
			}
			switch event.Action {
			case "start", "run", "pause", "cont", "bench":
				continue
			case "output":
				if strings.HasPrefix(strings.TrimSpace(event.Output), "--- ") {
					continue
				}
				stdout.WriteString(event.Output)
			case "pass", "fail", "skip":
				fmt.Fprintf(&stdout, "--- %s: %s\n", strings.ToUpper(event.Action), event.Test)
			}
		}
		if exitErr != nil {
			fmt.Fprintln(&stdout, exitErr.Error())
		}
		return stdout.String(), stderr.String(), err
	}, err
}
