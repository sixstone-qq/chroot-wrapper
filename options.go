package main

// Manage options in chroot-wrapper
import (
	"flag"
	"fmt"
	"os"
)

// Options are the arguments given from command line
type Options struct {
	// Subcommand to run
	Command string
	// Arguments passed to the subcommand
	Args []string
	// flagset
	flagset *flag.FlagSet
	// Listening port for the supervisor, having different ports
	// allowed us to have different tasks running at the same time
	ListeningPort int `cfg: "port"`
}

// Default options
const DefaultListeningPort = 6969

func (o *Options) Usage() {
	o.flagset.Usage()
}

func PrintSubcommandsUsage() {
	fmt.Fprintf(os.Stderr, "\t run URL|path cmd [args...]\n\n")
	fmt.Fprintf(os.Stderr, "\t\tRun cmd inside an image (jailed) which is available at the given URL.\n\t\tOnly file and HTTP(S) schemes are supported.\n\t\tOnly TAR images compressed or not with GZ are supported\n")
	fmt.Fprintf(os.Stderr, "\t ps\n\n")
	fmt.Fprintf(os.Stderr, "\t\tGet the status of task launched with run subcommand\n\n")
	fmt.Fprintf(os.Stderr, "\t kill [signal]\n\n")
	fmt.Fprintf(os.Stderr, "\t\tSend signal to the task launched with run subcommand\n")
	fmt.Fprintf(os.Stderr, "\t\tPossible signal values: SIGKILL (default), SIGTERM, SIGUSR1, SIGUSR2, SIGSTOP, SIGCONT, SIGINT\n")
}

func UserOptions() *Options {
	return setupUserOptions(os.Args[1:], flag.ExitOnError)
}

func setupUserOptions(args []string, errorHandling flag.ErrorHandling) *Options {
	opts := new(Options)

	flagSet := flag.NewFlagSet("chroot-wrapper", errorHandling)
	flagSet.Int("port", opts.ListeningPort, "Supervisor listening port to query task")
	flagSet.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage %s [flags] <subcommand> [arguments]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Available subcommands: run, ps, kill\n\n")
		PrintSubcommandsUsage()
		flagSet.PrintDefaults()
	}

	flagSet.Parse(args)

	// Retrieve subcommand to run
	opts.Command = flagSet.Arg(0)
	if len(flagSet.Args()) > 0 {
		opts.Args = flagSet.Args()[1:]
	}
	opts.flagset = flagSet

	listeningPort := flagSet.Lookup("port").Value.(flag.Getter).Get().(int)
	if listeningPort > 0 {
		opts.ListeningPort = listeningPort
	} else {
		opts.ListeningPort = DefaultListeningPort
	}

	return opts
}
