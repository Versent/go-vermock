package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/google/subcommands"
)

func main() {
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(&genCmd{}, "")
	flag.Parse()

	// Initialize the default logger to log to stderr.
	log.SetFlags(0)
	log.SetPrefix("mockgen: ")
	log.SetOutput(os.Stderr)

	ctx := context.Background()

	allCmds := map[string]bool{}
	subcommands.DefaultCommander.VisitCommands(func(_ *subcommands.CommandGroup, cmd subcommands.Command) { allCmds[cmd.Name()] = true })
	// Default to running the "gen" command.
	if args := flag.Args(); len(args) == 0 || !allCmds[args[0]] {
		genCmd := &genCmd{}
		os.Exit(int(genCmd.Execute(ctx, flag.CommandLine)))
	}
	os.Exit(int(subcommands.Execute(ctx)))
}
