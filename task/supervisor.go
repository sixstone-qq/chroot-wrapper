// Server to get information from a given task

package task

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"syscall"
)

// Supervisor is a HTTP server which serve requests on a stored task
type Supervisor struct {
	Task *Task
	HTTP *http.Server
}

type SupervisorQuery string

const (
	StatusQuery SupervisorQuery = "ps"
	SignalQuery SupervisorQuery = "kill"
)

const statusUnprocessableEntity = 422

type SignalPayload struct {
	Signal string `json:"signal"`
}

// Server side

// NewSupervisor creates a task supervisor from a task received from
// the channel
func NewSupervisor(tc <-chan *Task, listeningPort int) *Supervisor {
	s := &Supervisor{<-tc, nil}

	http.HandleFunc("/"+string(StatusQuery), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		jsonEnc := json.NewEncoder(w)
		if err := jsonEnc.Encode(s.Task.Status().String()); err != nil {
			panic(err)
		}
	})
	http.HandleFunc("/"+string(SignalQuery), func(w http.ResponseWriter, r *http.Request) {

		if r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			jsonDec := json.NewDecoder(r.Body)
			var payload SignalPayload
			if err := jsonDec.Decode(&payload); err != nil {
				w.WriteHeader(statusUnprocessableEntity)
				if err := json.NewEncoder(w).Encode(err); err != nil {
					panic(err)
				}
			}
			var signal os.Signal
			switch payload.Signal {
			case "SIGKILL":
				signal = os.Kill
			case "SIGINT":
				signal = os.Interrupt
			case "SIGSTOP":
				signal = syscall.SIGSTOP
			case "SIGCONT":
				signal = syscall.SIGCONT
			case "SIGTERM":
				signal = syscall.SIGTERM
			case "SIGUSR1":
				signal = syscall.SIGUSR1
			case "SIGUSR2":
				signal = syscall.SIGUSR2
			default:
				w.WriteHeader(http.StatusBadRequest)
				serr := "Invalid signal. Choices: SIGKILL, SIGINT, SIGSTOP, SIGTERM, SIGUSR1, SIGUSR2"
				jsonEnc := json.NewEncoder(w)
				if err := jsonEnc.Encode(serr); err != nil {
					panic(err)
				}
				return
			}
			if serr := s.Task.Signal(signal); serr != nil {
				panic(serr)
			}
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode("Signaled"); err != nil {
				panic(err)
			}
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(fmt.Sprintf("%d Method not allowed\n", http.StatusMethodNotAllowed)))
		}
	})

	s.HTTP = &http.Server{
		Addr: fmt.Sprintf(":%d", listeningPort),
	}

	return s
}

// ListenAndServe listens to receive queries from a given task
func (s *Supervisor) ListenAndServe() error {
	return s.HTTP.ListenAndServe()
}

// Client side

// QuerySupervisor asks for information and manage a running task
func QuerySupervisor(port int, query SupervisorQuery, args ...string) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/%s", port, query)
	switch query {
	case StatusQuery:
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
	case SignalQuery:
		byts, err := json.Marshal(&SignalPayload{args[0]})
		if err != nil {
			return fmt.Errorf("JSON marshalling: %v", err)
		}
		res, err := http.Post(url, "application/json", bytes.NewBuffer(byts))
		if err != nil {
			return fmt.Errorf("Cannot signal to task: %v", err)
		}
		defer res.Body.Close()
		if res.StatusCode == http.StatusOK {
			decJson := json.NewDecoder(res.Body)
			var status string
			err = decJson.Decode(&status)
			if err != nil {
				return err
			}
			fmt.Println("Task:", status)
		} else {
			response, _ := ioutil.ReadAll(res.Body)
			fmt.Fprintf(os.Stderr, "ERROR %s: %s", res.Status, response)
		}
	}
	return nil
}
