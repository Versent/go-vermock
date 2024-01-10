package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/google/subcommands"

	"github.com/Versent/go-vermock/internal/cmd/vermockgen"
)

func main() {
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(&vermockgen.GenCmd{}, "")

	// Initialize the default logger to log to stderr.
	log.SetFlags(0)
	log.SetPrefix(filepath.Base(os.Args[0]) + ": ")
	log.SetOutput(os.Stderr)

	ctx := context.Background()

	allCmds := map[string]bool{}
	subcommands.DefaultCommander.VisitCommands(func(_ *subcommands.CommandGroup, cmd subcommands.Command) { allCmds[cmd.Name()] = true })
	// Default to running the "gen" command.
	if args := os.Args[1:]; len(args) == 0 || !allCmds[args[0]] {
		f := flag.NewFlagSet("gen", flag.ContinueOnError)
		genCmd := vermockgen.NewGenCmd(nil, f)
		f.Usage = func() {
			cdr := subcommands.DefaultCommander
			cdr.ExplainCommand(cdr.Error, genCmd)
		}
		if f.Parse(args) != nil {
			os.Exit(int(subcommands.ExitUsageError))
		}
		os.Exit(int(genCmd.Execute(ctx, f)))
	}
	flag.Parse()
	os.Exit(int(subcommands.Execute(ctx)))
}
