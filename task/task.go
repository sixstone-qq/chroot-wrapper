package task

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
)

const TaskFilePrefix = "task"

var SupportedSchemes = map[string]bool{
	"":      true, // Empty scheme is translated to "file"
	"file":  true,
	"http":  true,
	"https": true,
}

type Task struct {
	// Command to execute
	Command string
	// URL the URL to an image which contains a FS
	URL *url.URL
	// temp file where the image is stored
	tempfile *os.File
}

// CreateTask creates a task by parsing a url.
// Current working URL schemes: file. Empty URL scheme implies file
func CreateTask(command string, rawurl string) (t *Task, err error) {
	URL, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	if !SupportedSchemes[URL.Scheme] {
		return nil, errors.New("Only file is the supported scheme")
	}
	if URL.Scheme == "" {
		URL.Scheme = "file"
	}
	t = new(Task)
	t.Command = command
	t.URL = URL
	return t, nil
}

func checkedClose(f io.Closer, err *error) {
	cerr := f.Close()
	if *err != nil {
		err = &cerr
	}
}

// Close removes everything we did in the system
func (t *Task) Close() {
	if t.tempfile != nil {
		os.Remove(t.tempfile.Name())
	}
}

func (t *Task) ImagePath() string {
	return t.tempfile.Name()
}

// Retrieve gets the URL from and it stored in the temporary directory
// as temporary file. See io/ioutil for details.
func (t *Task) Retrieve() (err error) {
	var src io.Reader
	switch t.URL.Scheme {
	case "file":
		if src, err = os.Open(t.URL.Path); err != nil {
			return err
		}
		defer checkedClose(src.(io.ReadCloser), &err)
	case "http", "https":
		resp, err := http.Get(t.URL.String())
		if err != nil {
			return err
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("Impossible to get %v: %v", t.URL.String(), resp.Status)
		}

		defer checkedClose(resp.Body, &err)
		src = resp.Body
	default:
		return fmt.Errorf("Invalid scheme %v", t.URL.Scheme)
	}

	if t.tempfile, err = ioutil.TempFile("", TaskFilePrefix); err != nil {
		return err
	}
	// Close the temporary file after the copy
	defer checkedClose(t.tempfile, &err)

	if _, err = io.Copy(t.tempfile, src); err != nil {
		return err
	}

	// I didn't manage to do that before downloading the whole
	// file because of limitations in compress package to use with
	// bufio.Reader
	// Check if the image is a valid archive
	err = IsValidImage(t.tempfile.Name())

	return err
}
