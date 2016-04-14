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
	var err error
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

		done := make(chan struct{})
		tc := make(chan *task.Task)
		go func(taskChan chan *task.Task, end chan struct{}) {
			defer close(end)
			defer close(taskChan)
			task, err := task.CreateTask(opts.Args[0], opts.Args[1], opts.Args[2:]...)
			if err != nil {
				log.Fatalf("Impossible to create task: %v", err)
			}
			defer task.Close()
			taskChan <- task

			err = task.StartChroot()
			if err != nil {
				log.Fatalf("Impossible to start task: %v", err)
			}

			if err = task.Command.Wait(); err != nil {
				log.Printf("ERROR: waiting for the task: %v", err)
			}
		}(tc, done)

		go func() {
			supervisor := task.NewSupervisor(tc, opts.ListeningPort)
			// It is ended by main goroutine when it exits
			if serr := supervisor.ListenAndServe(); serr != nil {
				log.Printf("WARN: Supervisor cannot listen at %d: %s", opts.ListeningPort, serr)
				log.Printf("WARN: No possible to query the task later")
			}
		}()

		// Wait for the task to exit
		<-done
	case "ps":
		if err = task.QuerySupervisor(opts.ListeningPort, task.StatusQuery); err != nil {

			err = fmt.Errorf("Error querying task status: %v", err)
		}
	case "kill":
		var signal string
		if len(opts.Args) >= 1 {
			signal = opts.Args[0]
		} else {
			signal = "SIGKILL"
		}

		if err = task.QuerySupervisor(opts.ListeningPort, task.SignalQuery, signal); err != nil {
			err = fmt.Errorf("Error sending signal to task: %v", err)
		}
	default:
		fmt.Fprintf(os.Stderr, "Missing subcommand parameter, available subcommands:\n\n")
		fmt.Fprintf(os.Stderr, "  run, ps, kill\n")
	}
	if err != nil {
		if opts.Command == "ps" || opts.Command == "kill" {
			// Give some hint
			fmt.Fprintf(os.Stderr, "%s\nIs task running or in a different port?\n", err)
		}
		os.Exit(1)
	}
}
