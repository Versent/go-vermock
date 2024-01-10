package mock_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rsc.io/script"
	"rsc.io/script/scripttest"

	vermockgen "github.com/Versent/go-vermock/internal/cmd/vermockgen"
	"github.com/Versent/go-vermock/internal/mock"
)

func TestGenerate(t *testing.T) {
	engine := script.NewEngine()
	engine.Cmds["vermockgen"] = &genCmd{}
	mutdir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	mutdir = filepath.Join(mutdir, "..", "..")
	scripttest.Test(
		t,
		context.Background(),
		engine,
		[]string{"MUT=" + mutdir},
		"testdata/*.txt",
	)
}

type genCmd struct{}

func (m *genCmd) Run(s *script.State, args ...string) (script.WaitFunc, error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	f := flag.NewFlagSet("gen", flag.ContinueOnError)
	f.SetOutput(stderr)
	err := f.Parse(args)
	if err != nil {
		return nil, err
	}
	l := log.New(stderr, "vermockgen: ", 0)
	genCmd := vermockgen.NewGenCmd(l, f)
	status := genCmd.Execute(s.Context(), f, mock.WithDir(s.Getwd()))
	return func(s *script.State) (_, _ string, err error) {
		if status != 0 {
			err = fmt.Errorf("exit status %d", status)
		}
		return stdout.String(), stderr.String(), err
	}, nil
}

func (m *genCmd) Usage() *script.CmdUsage {
	genCmd := &vermockgen.GenCmd{}
	usage := strings.Split(genCmd.Usage(), "\n")
	args, detail := usage[0], usage[1:]
	for len(detail) > 0 && detail[0] == "" {
		detail = detail[1:]
	}
	return &script.CmdUsage{
		Summary: genCmd.Synopsis(),
		Args:    args,
		Detail:  detail,
	}
}
