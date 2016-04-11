package task

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
)

const TaskFilePrefix = "task"

var SupportedSchemes = map[string]bool{
	"":      true, // Empty scheme is translated to "file"
	"file":  true,
	"http":  true,
	"https": true,
}

// Task is a command + URL to an image
type Task struct {
	// Command to execute. It can be used to retrieve the results
	Command *exec.Cmd
	// URL the URL to an image which contains a FS
	URL *url.URL
	// temp file where the image is stored
	image *os.File
	// compressed?
	compressed bool
	// extracted image directory
	dirimage string
}

// CreateTask creates a task by parsing a url.
// Current working URL schemes: file. Empty URL scheme implies file
func CreateTask(rawurl string, command string, args ...string) (t *Task, err error) {
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
	t = &Task{
		Command: exec.Command(command, args...),
		URL:     URL,
	}
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
	if len(t.dirimage) > 0 {
		os.RemoveAll(t.dirimage)
	}
	if t.image != nil {
		os.Remove(t.image.Name())
	}
}

// ImagePath returns the path where the image file is stored
func (t *Task) ImagePath() string {
	return t.image.Name()
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

	if t.image, err = ioutil.TempFile("", TaskFilePrefix); err != nil {
		return err
	}
	// Close the temporary file after the copy
	defer checkedClose(t.image, &err)

	if _, err = io.Copy(t.image, src); err != nil {
		return err
	}

	// I didn't manage to do that before downloading the whole
	// file because of limitations in compress package to use with
	// bufio.Reader
	// Check if the image is a valid archive and it is compressed
	t.compressed, err = ValidImage(t.image.Name())

	return err
}

// Start the command asynchronously
func (t *Task) Start() (err error) {
	if t.image == nil {
		if err = t.Retrieve(); err != nil {
			return err
		}
		// Extract the content in dirimage
		if t.dirimage, err = ioutil.TempDir("", TaskFilePrefix); err != nil {
			return err
		}
		if err = t.extractImage(); err != nil {
			return err
		}
	}
	return t.Command.Start()
}

// Extract a image in the dirimage
func (t *Task) extractImage() error {
	var reader io.Reader

	image, err := os.OpenFile(t.image.Name(), os.O_RDONLY, 0444)
	if err != nil {
		return err
	}
	defer checkedClose(image, &err)

	if t.compressed {
		if reader, err = gzip.NewReader(image); err != nil {
			return err
		}
		defer checkedClose(reader.(io.Closer), &err)
	} else {
		reader = image
	}
	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// End of the tar archive
			break
		} else if err != nil {
			return err
		}
		path := hdr.Name
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(path, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(path, os.O_RDWR|os.O_TRUNC, 0777)
			if err != nil {
				if f, err = os.Create(path); err != nil {
					return err
				}
			}
			defer checkedClose(f, &err)
			if _, err = io.Copy(f, tr); err != nil {
				return err
			}
		default:
			return fmt.Errorf("Unknown type flag %c for path %s", hdr.Typeflag, path)
		}
	}

	return err
}
