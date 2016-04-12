package main

import (
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
	// URL Command Args
	task, err := task.CreateTask(os.Args[1], os.Args[2], os.Args[3:]...)
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
}
