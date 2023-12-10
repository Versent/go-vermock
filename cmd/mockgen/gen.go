package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/subcommands"

	"github.com/Versent/go-mock/internal/mock"
)

// packages returns the slice of packages to run mockgen over based on f.
// It defaults to ".".
func packages(f *flag.FlagSet) []string {
	pkgs := f.Args()
	if len(pkgs) == 0 {
		pkgs = []string{"."}
	}
	return pkgs
}

// makeGenerateOptions returns an initialized mock.GenerateOptions, possibly
// with the Header option set.
func makeGenerateOptions(headerFile string) (opts mock.GenerateOptions, err error) {
	opts.Dir, err = os.Getwd()
	if err != nil {
		err = fmt.Errorf("failed to get working directory: %v", err)
		return
	}

	opts.Env = os.Environ()

	if headerFile != "" {
		opts.Header, err = os.ReadFile(headerFile)
	}
	if err != nil {
		err = fmt.Errorf("failed to read header file %q: %v", headerFile, err)
		return
	}

	return
}

type genCmd struct {
	log            *log.Logger
	headerFile     string
	prefixFileName string
	tags           string
}

func NewGenCmd(l *log.Logger, f *flag.FlagSet) *genCmd {
	cmd := &genCmd{log: l}
	cmd.SetFlags(f)
	return cmd
}

func (*genCmd) Name() string { return "gen" }
func (*genCmd) Synopsis() string {
	return "generate the mock_gen.go file for each package"
}
func (*genCmd) Usage() string {
	return `gen [-header file] [-tags buildtags] [package ...]

  Given one or more packages, gen creates mock_gen.go files for each.

  If no package is listed, it defaults to ".".

`
}
func (cmd *genCmd) SetFlags(f *flag.FlagSet) {
	if cmd.log == nil {
		cmd.log = log.Default()
	}
	f.StringVar(&cmd.headerFile, "header", "", "path to file to insert as a header in mock_gen.go")
	f.StringVar(&cmd.tags, "tags", "", "append build tags to the default mockstub")
}

func (cmd *genCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	opts, err := makeGenerateOptions(cmd.headerFile)
	if err != nil {
		cmd.log.Println(err)
		return subcommands.ExitFailure
	}

	opts.PrefixOutputFile = cmd.prefixFileName
	opts.Tags = cmd.tags

	outs, errs := mock.Generate(ctx, packages(f), opts)
	if len(errs) > 0 {
		logErrors(cmd.log, errs...)
		cmd.log.Println("generate failed")
		return subcommands.ExitFailure
	}
	if len(outs) == 0 {
		return subcommands.ExitSuccess
	}
	success := true
	for _, out := range outs {
		if len(out.Errs) > 0 {
			logErrors(cmd.log, out.Errs...)
			cmd.log.Printf("%s: generate failed\n", out.PkgPath)
			success = false
		}
		if len(out.Content) == 0 {
			// No output. Maybe errors, maybe no directives.
			continue
		}
		if err := out.Commit(); err == nil {
			cmd.log.Printf("%s: wrote %s\n", out.PkgPath, out.OutputPath)
		} else {
			cmd.log.Printf("%s: failed to write %s: %v\n", out.PkgPath, out.OutputPath, err)
			success = false
		}
	}
	if !success {
		cmd.log.Println("at least one generate failure")
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}
