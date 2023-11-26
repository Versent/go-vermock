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
	headerFile     string
	prefixFileName string
	tags           string
}

func (*genCmd) Name() string { return "gen" }
func (*genCmd) Synopsis() string {
	return "generate the mock_gen.go file for each package"
}
func (*genCmd) Usage() string {
	return `gen [packages]

  Given one or more packages, gen creates mock_gen.go files for each.

  If no packages are listed, it defaults to ".".
`
}
func (cmd *genCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&cmd.headerFile, "header_file", "", "path to file to insert as a header in mock_gen.go")
	f.StringVar(&cmd.tags, "tags", "", "append build tags to the default mockstub")
}

func (cmd *genCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	opts, err := makeGenerateOptions(cmd.headerFile)
	if err != nil {
		log.Println(err)
		return subcommands.ExitFailure
	}

	opts.PrefixOutputFile = cmd.prefixFileName
	opts.Tags = cmd.tags

	outs, errs := mock.Generate(ctx, packages(f), opts)
	if len(errs) > 0 {
		logErrors(errs...)
		log.Println("generate failed")
		return subcommands.ExitFailure
	}
	if len(outs) == 0 {
		return subcommands.ExitSuccess
	}
	success := true
	for _, out := range outs {
		if len(out.Errs) > 0 {
			logErrors(out.Errs...)
			log.Printf("%s: generate failed\n", out.PkgPath)
			success = false
		}
		if len(out.Content) == 0 {
			// No output. Maybe errors, maybe no directives.
			continue
		}
		if err := out.Commit(); err == nil {
			log.Printf("%s: wrote %s\n", out.PkgPath, out.OutputPath)
		} else {
			log.Printf("%s: failed to write %s: %v\n", out.PkgPath, out.OutputPath, err)
			success = false
		}
	}
	if !success {
		log.Println("at least one generate failure")
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}
