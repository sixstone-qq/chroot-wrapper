package task

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

func TestCreateTask(testSuite *testing.T) {
	var tests = []struct {
		url        string
		shouldFail bool
	}{
		// Valid case (we assume url.Parse works)
		{"file:///tmp/image", false},
		{":[/]ralara", true},
		{"ftp://foo.com/bar", true},
	}

	for _, test := range tests {
		t, err := CreateTask(test.url, "cmd")
		if err == nil && test.shouldFail {
			testSuite.Errorf("The url %q found unexpected error: %v", test.url, err)
		}
		if t != nil {
			defer t.Close()
		}
	}
}

func TestFailFileRetrieve(test *testing.T) {
	src, err := ioutil.TempFile("", "")
	defer src.Close()
	if err != nil {
		test.Fatalf("Impossible to create a temp file %v", err)
	}
	fileURL := new(url.URL)
	fileURL.Scheme = "file"
	fileURL.Path = src.Name()
	testRetrieve(test, fileURL.String(), true)
}

func TestGZTarFileRetrieve(test *testing.T) {
	src, err := ioutil.TempFile("", "")
	if err != nil {
		test.Fatalf("Impossible to create a temp file %v", err)
	}
	defer os.Remove(src.Name())
	err = createGZTarContent(src, test)

	if err = src.Close(); err != nil {
		test.Fatalf("Error closing file: %v", err)
	}

	fileURL := &url.URL{
		Scheme: "file",
		Path:   src.Name(),
	}
	testRetrieve(test, fileURL.String(), false)
}

func TestFailHTTPRetrieve(test *testing.T) {
	// Create a new HTTP server
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "Hi!")
		}))
	defer ts.Close()
	testRetrieve(test, ts.URL, true)
}

func TestStart(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			createGZTarContent(w, t)
		}))
	defer ts.Close()

	// Test chroot case only if possible
	var cases = []bool{false, true}

	for _, chrooted := range cases {
		task, err := CreateTask(ts.URL, "pwd")
		if err != nil {
			t.Fatalf("Error creating task: %v", err)
		}

		stdout, err := task.Command.StdoutPipe()
		if err != nil {
			t.Fatalf("Error creating stdout pipe: %v", err)
		}

		if chrooted && os.Geteuid() != 0 {
			t.Log("Start a task in a chroot jail only possible with root")
			break
		}

		if chrooted {
			err = task.StartChroot()
			if err == nil {
				// This should fail
				t.Error("Chroot tests must fail")
				continue
			} else {
				// We cannot more here
				task.Close()
				continue
			}
		} else {
			err = task.Start()
		}
		if err != nil {
			t.Errorf("Error starting a task: %v", err)
			continue
		}

		bytes, err := ioutil.ReadAll(stdout)
		if err != nil {
			t.Errorf("Gathering output: %v", err)
			continue
		}
		t.Logf("Result from command: %s", bytes)

		if err = task.Command.Wait(); err != nil {
			t.Errorf("Error waiting for the task: %v", err)
			continue
		}
		task.Close()
	}
}

// Helper functions

// Helper function to use for different backends
func testRetrieve(test *testing.T, rawurl string, shouldFail bool) {
	t, err := CreateTask(rawurl, "cmd")
	if err != nil {
		test.Fatalf("Impossible to create a task: %v", err)
	}
	defer t.Close()
	err = t.Retrieve()
	if shouldFail && err == nil {
		test.Fatalf("It should fail on retrieving the task")
	}
	if !shouldFail {
		if err != nil {
			test.Fatalf("Error retrieving a task: %v", err)
		}
		if t.ImagePath() == "" {
			test.Fatalf("Not possible to get the image path")
		}
		test.Logf("Image path: %q", t.ImagePath())
	}
}

// Helper to create a TAR GZ file
func createGZTarContent(src io.Writer, test *testing.T) (err error) {
	var files = []struct {
		Name, Body string
	}{
		{"readme.txt", "This archive contains this file."},
	}

	gw := gzip.NewWriter(src)
	tw := tar.NewWriter(gw)
	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.Name,
			Mode:     0600,
			Size:     int64(len(file.Body)),
			Typeflag: byte('0'),
		}
		if err = tw.WriteHeader(hdr); err != nil {
			test.Fatalf("Impossible to write TAR header: %v", err)
		}
		if _, err = tw.Write([]byte(file.Body)); err != nil {
			test.Fatalf("Impossible to write file %q to TAR: %v", file.Name, err)
		}
	}
	// Make sure to check the error on Close
	if err = gw.Close(); err != nil {
		test.Fatalf("Error closing GZ file: %v", err)
	}
	if err = tw.Close(); err != nil {
		test.Fatalf("Error closing TAR GZ file: %v", err)
	}
	return
}
