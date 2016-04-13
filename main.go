package main

import (
	"fmt"
	"log"
	"os"

	"github.com/sixstone-qq/chroot-wrapper/task"
)

func main() {
	if os.Args[0] == task.TaskForkName {
		// Create the view of the system and exec
		if err := task.RunContainer(); err != nil {
			log.Fatalf("Run container error: %v", err)
		}
		os.Exit(0)
	}
	// Standard call
	opts := UserOptions()

	switch opts.Command {
	case "run":
		// URL Command Args
		if len(opts.Args) < 2 {
			switch len(opts.Args) {
			case 0:
				fmt.Fprintf(os.Stderr, "Missing URL and command to run\n")
			case 1:
				fmt.Fprintf(os.Stderr, "Missing Command to run\n")
			}
			opts.Usage()
			break
		}
		task, err := task.CreateTask(opts.Args[0], opts.Args[1], opts.Args[2:]...)
		if err != nil {
			log.Fatalf("Impossible to create task: %v", err)
		}
		defer task.Close()

		err = task.StartChroot()
		if err != nil {
			log.Fatalf("Impossible to start task: %v", err)
		}

		if err = task.Command.Wait(); err != nil {
			log.Fatalf("Error waiting for the task: %v", err)
		}
	default:
		fmt.Fprintf(os.Stderr, "Missing subcommand parameter, available subcommands:\n\n")
		fmt.Fprintf(os.Stderr, "  run\n")
	}
}
