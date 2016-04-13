// Server to get information from a given task

package task

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Supervisor struct {
	Task *Task
	HTTP *http.Server
}

type SupervisorQuery string

const (
	StatusQuery SupervisorQuery = "ps"
)

// NewSupervisor creates a task supervisor from a task received from
// the channel
func NewSupervisor(tc <-chan *Task) *Supervisor {
	s := &Supervisor{<-tc, nil}

	http.HandleFunc("/"+string(StatusQuery), func(w http.ResponseWriter, r *http.Request) {
		jsonEnc := json.NewEncoder(w)
		jsonEnc.Encode(s.Task.Status().String())
	})

	s.HTTP = &http.Server{
		Addr: ":6969",
	}

	return s
}

// ListenAndServe listens to receive queries from a given task
func (s *Supervisor) ListenAndServe() error {
	return s.HTTP.ListenAndServe()
}

// QuerySupervisor asks for information and manage a running task
func QuerySupervisor(query SupervisorQuery) error {
	url := fmt.Sprintf("http://127.0.0.1:6969/%s", query)
	res, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Cannot get the task status: %v", err)
	}
	defer res.Body.Close()
	decJson := json.NewDecoder(res.Body)
	var status string
	err = decJson.Decode(&status)
	if err != nil {
		return err
	}
	fmt.Println("Task status:", status)
	return nil
}
