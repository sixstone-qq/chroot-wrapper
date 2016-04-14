package task

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

const (
	TaskFilePrefix = "task"
	TaskForkName   = "tfork"
)

// Status of a task
type Status int

const (
	NotStarted Status = iota
	Retrieved
	Extracted
	Running
	Stopped
	Sleeping
	Zombie
	Finished
)

// String representation of the task status
var statusStrs = [...]string{"NotStarted", "Retrieved", "Extracted", "Running", "Stopped", "Sleeping", "Zombie", "Finished"}

func (s Status) String() string {
	return statusStrs[s]
}

var SupportedSchemes = map[string]bool{
	"":      true, // Empty scheme is translated to "file"
	"file":  true,
	"http":  true,
	"https": true,
}

// Task is a command + URL to an image
type Task struct {
	sync.RWMutex
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
	t.RLock()
	defer t.RUnlock()
	if len(t.dirimage) > 0 {
		os.RemoveAll(t.dirimage)
	}
	if t.image != nil {
		os.Remove(t.image.Name())
	}
}

// ImagePath returns the path where the image file is stored
func (t *Task) ImagePath() string {
	t.RLock()
	name := t.image.Name()
	t.RUnlock()
	return name
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
		if resp.StatusCode != http.StatusOK {
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

// Start the command asynchronously with wd as working directory and
// env with the environment variables
func (t *Task) Start(wd string, env []string) error {
	return t.start(false, wd, env)
}

func (t *Task) start(chrooted bool, wd string, env []string) (err error) {
	t.Lock()
	defer t.Unlock()
	if t.image == nil {
		if err = t.Retrieve(); err != nil {
			return err
		}
	}
	if len(t.dirimage) == 0 {
		// Extract the content in dirimage
		if err = t.extractImage(); err != nil {
			t.dirimage = ""
			return err
		}
	}
	t.Command.Dir = t.dirimage
	if chrooted {
		// FIXME: Check Linux
		// Check the caps
		if os.Geteuid() == 0 {
			t.Command.SysProcAttr = &syscall.SysProcAttr{Chroot: t.dirimage}
		} else {
			// Use unprivileged mode
			// By calling the same program with different arguments
			// See libcontainer doc for details
			args := []string{TaskForkName}
			if wd != "" {
				args = append(args, "-wd", wd)
			}
			t.Command.Args = append(args, t.Command.Args...)
			t.Command.Path = "/proc/self/exe"
			t.Command.SysProcAttr = &syscall.SysProcAttr{
				Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
				UidMappings: []syscall.SysProcIDMap{
					{
						ContainerID: 0,
						HostID:      os.Geteuid(),
						Size:        1,
					},
				},
				GidMappings: []syscall.SysProcIDMap{
					{
						ContainerID: 0,
						HostID:      os.Getegid(),
						Size:        1,
					},
				},
			}
			t.Command.Stdin = os.Stdin
			t.Command.Stdout = os.Stdout
			t.Command.Stderr = os.Stderr
		}
	} else if wd != "" {
		t.Command.Dir = wd
	}

	if len(env) > 0 {
		t.Command.Env = env
	}
	return t.Command.Start()
}

// StartChroot starts the command asynchronously in the chroot jail.
// In Linux, it uses pivot_root to avoid scaling privileges
// Pass environment variables from env and working directory set to wd
func (t *Task) StartChroot(wd string, env []string) error {
	err := t.start(true, wd, env)
	if err == nil {
		log.Println("Container PID: ", t.Command.Process.Pid)
	}
	return err
}

// Status returns the current status of the task
func (t *Task) Status() Status {
	t.RLock()
	defer t.RUnlock()
	status := NotStarted
	if t.image != nil {
		status = Retrieved
	}
	if len(t.dirimage) > 0 {
		status = Extracted
	}
	if t.Command.Process != nil {
		p, err := os.FindProcess(t.Command.Process.Pid)
		if err == nil {
			if p != nil {
				if t.Command.ProcessState == nil {
					state, _ := procPidStat(t.Command.Process.Pid)
					switch state {
					case 'T':
						status = Stopped
					case 'S':
						status = Sleeping
					case 'Z':
						status = Zombie
					default:
						status = Running
					}
				} else {
					status = Finished
				}
			}
		}
	}
	return status
}

// Get the real state of the running process using `proc` FS
// It can return RSDZTW as stated by man 5 proc
func procPidStat(pid int) (rune, error) {
	filename := filepath.Join(string(filepath.Separator), "proc", strconv.FormatInt(int64(pid), 10), "stat")
	f, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	var p int
	var procname string
	var state rune
	fmt.Fscanf(f, "%d %s %c", &p, &procname, &state)
	return state, nil
}

// Signal a task with the given signal
func (t *Task) Signal(sig os.Signal) error {
	t.RLock()
	defer t.RUnlock()
	if t.Command.Process != nil {
		return t.Command.Process.Signal(sig)
	}
	return fmt.Errorf("Impossible to send a signal to a non-running process")
}

// Extract a image in the dirimage
func (t *Task) extractImage() (err error) {
	var reader io.Reader

	if t.dirimage, err = ioutil.TempDir("", TaskFilePrefix); err != nil {
		return fmt.Errorf("TempDir: %v", err)
	}

	image, err := os.OpenFile(t.image.Name(), os.O_RDONLY, 0444)
	if err != nil {
		return
	}
	defer checkedClose(image, &err)

	if t.compressed {
		if reader, err = gzip.NewReader(image); err != nil {
			return
		}
		defer checkedClose(reader.(io.Closer), &err)
	} else {
		reader = image
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Getwd: %v", err)
	}
	if err = os.Chdir(t.dirimage); err != nil {
		return fmt.Errorf("Chdir: %v", err)
	}
	defer func() {
		cherr := os.Chdir(wd)
		if err == nil && cherr != nil {
			err = cherr
		}
	}()

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
			if err = os.Chmod(path, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("Chmod: %v", err)
			}
			if _, err = io.Copy(f, tr); err != nil {
				return fmt.Errorf("Copy: %v", err)
			}
		case tar.TypeSymlink:
			target := hdr.Linkname
			if filepath.IsAbs(hdr.Linkname) {
				target = strings.TrimPrefix(hdr.Linkname, string(filepath.Separator))
				target = filepath.Join(t.dirimage, target)
				abspath := filepath.Dir(filepath.Join(t.dirimage, path))
				if target, err = filepath.Rel(abspath, target); err != nil {
					return err
				}
			}
			if err = os.Symlink(target, path); err != nil {
				switch err.(*os.LinkError).Err.Error() {
				case "file exists": // Do nothing
				case "no such file or directory":
					// Create the target directory file first
					if err = os.MkdirAll(filepath.Dir(target), 0777); err != nil {
						return err
					}
					if err = os.Symlink(target, path); err != nil {
						return fmt.Errorf("Symlink: %v", err)
					}
				default:
					return fmt.Errorf("Symlink: %v", err)
				}
			}
		default:
			return fmt.Errorf("Unknown type flag %c for path %s", hdr.Typeflag, path)
		}
	}
	return
}

// RunContainer sets up the view of the filesystem in namespaces and then run
func RunContainer(wd string) error {
	container := &Container{Args: os.Args[flag.NFlag()*2+1:]}
	return container.Run(wd)
}
