/*
Copyright 2014 Tamás Gulácsi.
Copyright 2013 The Camlistore Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package cmdmain contains the shared implementation for command-line tools.
// It is copied from Camlistore, where it is used for camget,
// camput, camtool, and other Camlistore command-line tools.
package cmdmain

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/tgulacsi/go/legal"
)

var (
	FlagHelp    = flag.Bool("help", false, "print usage")
	FlagVerbose = flag.Bool("verbose", false, "extra debug logging")
)

var (
	// ExtraFlagRegistration allows to add more flags from
	// other packages (with AddFlags) when Main starts.
	ExtraFlagRegistration = func() {}
	// PreExit runs after the subcommand, but before Main terminates
	// with either success or the error from the subcommand.
	PreExit = func() {}
	// ExitWithFailure determines whether the command exits
	// with a non-zero exit status.
	ExitWithFailure bool
	// CheckCwd checks the current working directory, and possibly
	// changes it, or aborts the run if needed.
	CheckCwd = func() {}
)

var ErrUsage = UsageError("invalid command")

type UsageError string

func (ue UsageError) Error() string {
	return "Usage error: " + string(ue)
}

var (
	// mode name to actual subcommand mapping
	modeCommand = make(map[string]CommandRunner)
	modeFlags   = make(map[string]*flag.FlagSet)
	wantHelp    = make(map[string]*bool)

	// Indirections for replacement by tests
	Stderr io.Writer = os.Stderr
	Stdout io.Writer = os.Stdout
	Stdin  io.Reader = os.Stdin

	Exit = realExit
	// TODO: abstract out vfs operation. should never call os.Stat, os.Open, os.Create, etc.
	// Only use fs.Stat, fs.Open, where vs is an interface type.
	// TODO: switch from using the global flag FlagSet and use our own. right now
	// running "go test -v" dumps the flag usage data to the global stderr.
)

func realExit(code int) {
	os.Exit(code)
}

// CommandRunner is the type that a command mode should implement.
type CommandRunner interface {
	Usage()
	RunCommand(args []string) error
}

type exampler interface {
	Examples() []string
}

type describer interface {
	Describe() string
}

// RegisterCommand adds a mode to the list of modes for the Main command.
// It is meant to be called in init() for each subcommand.
func RegisterCommand(mode string, makeCmd func(Flags *flag.FlagSet) CommandRunner) {
	if _, dup := modeCommand[mode]; dup {
		log.Fatalf("duplicate command %q registered", mode)
	}
	flags := flag.NewFlagSet(mode+" options", flag.ContinueOnError)
	flags.Usage = func() {}

	var cmdHelp bool
	flags.BoolVar(&cmdHelp, "help", false, "Help for this mode.")
	wantHelp[mode] = &cmdHelp
	modeFlags[mode] = flags
	modeCommand[mode] = makeCmd(flags)
}

func hasFlags(flags *flag.FlagSet) bool {
	any := false
	flags.VisitAll(func(*flag.Flag) {
		any = true
	})
	return any
}

// Errorf prints to Stderr
func Errorf(format string, args ...interface{}) {
	fmt.Fprintf(Stderr, format, args...)
}

func usage(msg string) {
	cmdName := filepath.Base(os.Args[0])
	if msg != "" {
		Errorf("Error: %v\n", msg)
	}
	Errorf(`
Usage: ` + cmdName + ` [globalopts] <mode> [commandopts] [commandargs]

Modes:

`)
	for mode, cmd := range modeCommand {
		if des, ok := cmd.(describer); ok {
			Errorf("  %s: %s\n", mode, des.Describe())
		}
	}
	Errorf("\nExamples:\n")
	for mode, cmd := range modeCommand {
		if ex, ok := cmd.(exampler); ok {
			exs := ex.Examples()
			if len(exs) > 0 {
				Errorf("\n")
			}
			for _, example := range exs {
				Errorf("  %s %s %s\n", cmdName, mode, example)
			}
		}
	}

	Errorf(`
For mode-specific help:

  ` + cmdName + ` <mode> -help

Global options:
`)
	flag.PrintDefaults()
	Exit(1)
}

func help(mode string) {
	cmdName := os.Args[0]
	// We can skip all the checks as they're done in Main
	cmd := modeCommand[mode]
	cmdFlags := modeFlags[mode]
	cmdFlags.SetOutput(Stderr)
	if des, ok := cmd.(describer); ok {
		Errorf("%s\n", des.Describe())
	}
	Errorf("\n")
	cmd.Usage()
	if hasFlags(cmdFlags) {
		cmdFlags.PrintDefaults()
	}
	if ex, ok := cmd.(exampler); ok {
		Errorf("\nExamples:\n")
		for _, example := range ex.Examples() {
			Errorf("  %s %s %s\n", cmdName, mode, example)
		}
	}
}

// registerFlagOnce guards ExtraFlagRegistration. Tests may invoke
// Main multiple times, but duplicate flag registration is fatal.
var registerFlagOnce sync.Once

// Main is meant to be the core of a command that has subcommands (modes).
// Chooses the matching subcommand, and runs it.
// Call CheckCwd on start, and PreExit before exiting.
func Main() {
	MainDefault("")
}

// MainDefault is meant to be the core of a command that has subcommands (modes).
// Chooses the matching subcommand, and runs it.
// Call CheckCwd on start, and PreExit before exiting.
//
// defaultMode will be used if not arguments given on command line.
func MainDefault(defaultMode string) {
	registerFlagOnce.Do(ExtraFlagRegistration)
	flag.CommandLine.SetOutput(Stderr)
	flag.Parse()
	CheckCwd()

	args := flag.Args()
	if *FlagHelp {
		usage("")
	}
	if legal.MaybePrint(Stderr) {
		return
	}
	if len(args) == 0 {
		if defaultMode == "" {
			usage("No mode given.")
		}
		args = append(args, defaultMode)
	}

	mode := args[0]
	cmd, ok := modeCommand[mode]
	if !ok {
		usage(fmt.Sprintf("Unknown mode %q", mode))
	}

	cmdFlags := modeFlags[mode]
	cmdFlags.SetOutput(Stderr)
	err := cmdFlags.Parse(args[1:])
	if err != nil {
		err = ErrUsage
	} else {
		if *wantHelp[mode] {
			help(mode)
			return
		}
		err = cmd.RunCommand(cmdFlags.Args())
	}
	var ue UsageError
	if errors.As(err, &ue) {
		Errorf("%s\n", ue)
		cmd.Usage()
		Errorf("\nGlobal options:\n")
		flag.PrintDefaults()

		if hasFlags(cmdFlags) {
			Errorf("\nMode-specific options for mode %q:\n", mode)
			cmdFlags.PrintDefaults()
		}
		Exit(1)
	}
	PreExit()
	if err != nil {
		if !ExitWithFailure {
			// because it was already logged if ExitWithFailure
			log.Printf("Error: %v", err)
		}
		Exit(2)
	}
}
