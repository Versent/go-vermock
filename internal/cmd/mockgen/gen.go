package mockgen

import (
	"context"
	"flag"
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

type GenCmd struct {
	log            *log.Logger
	headerFile     string
	prefixFileName string
	tags           string
}

func NewGenCmd(l *log.Logger, f *flag.FlagSet) *GenCmd {
	cmd := &GenCmd{log: l}
	cmd.SetFlags(f)
	return cmd
}

func (*GenCmd) Name() string { return "gen" }
func (*GenCmd) Synopsis() string {
	return "generate the mock_gen.go file for each package"
}
func (*GenCmd) Usage() string {
	return `gen [-header file] [-tags buildtags] [package ...]

  Given one or more packages, gen creates mock_gen.go files for each.

  If no package is listed, it defaults to ".".

`
}
func (cmd *GenCmd) SetFlags(f *flag.FlagSet) {
	if cmd.log == nil {
		cmd.log = log.Default()
	}
	f.StringVar(&cmd.headerFile, "header", "", "path to file to insert as a header in mock_gen.go")
	f.StringVar(&cmd.tags, "tags", "", "append build tags to the default mockstub")
}

func (cmd *GenCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...any) subcommands.ExitStatus {
	var opts mock.GenerateOptions
	err := mock.WithArgs(
		mock.WithEnv(os.Environ()),
		mock.WithHeaderFile(cmd.headerFile),
		mock.WithArgs(args...),
		mock.WithWDFallback(),
		mock.WithPrefixFileName(cmd.prefixFileName),
		mock.WithTags(cmd.tags),
	)(&opts)
	if err != nil {
		cmd.log.Println(err)
		return subcommands.ExitFailure
	}

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
