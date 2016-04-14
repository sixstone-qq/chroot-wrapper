package main

// Manage options in chroot-wrapper
import (
	"flag"
	"fmt"
	"os"
	"strings"
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
	// Environment variables to pass to the task
	env map[string]string `cfg: "env"`
	// Working directory for the task
	Dir string
}

// Default options
const DefaultListeningPort = 6969

// Usage prints usage from the options
func (o *Options) Usage() {
	o.flagset.Usage()
}

// Environ returns the environment variables from command line flags
// in key=value form
func (o *Options) Environ() []string {
	res := make([]string, len(o.env))
	for k, v := range o.env {
		res = append(res, fmt.Sprintf("%s=%s", k, v))
	}
	return res
}

func PrintSubcommandsUsage() {
	fmt.Fprintf(os.Stderr, "\t [-env=[]|-wd] run URL|path cmd [args...]\n\n")
	fmt.Fprintf(os.Stderr, "\t\tRun cmd inside an image (jailed) which is available at the given URL.\n\t\tOnly file and HTTP(S) schemes are supported.\n\t\tOnly TAR images compressed or not with GZ are supported\n\n")
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
	flagSet.String("env", "", "New environment variables available for the task")
	flagSet.String("wd", "", "Working directory to run the task")
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

	envArg := flagSet.Lookup("env").Value.String()
	if envArg != "" {
		opts.env = make(map[string]string)
		for _, kv := range strings.Split(envArg, ",") {
			kvs := strings.SplitN(kv, "=", 2)
			if len(kvs) < 2 {
				kvs = append(kvs, "")
			}
			opts.env[kvs[0]] = kvs[1]
		}
	}
	opts.Dir = flagSet.Lookup("wd").Value.String()

	return opts
}
