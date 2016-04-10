package task

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		t, err := CreateTask("cmd", test.url)
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
		test.Errorf("Impossible to create a temp file %v", err)
	}
	fileURL := new(url.URL)
	fileURL.Scheme = "file"
	fileURL.Path = src.Name()
	testRetrieve(test, fileURL.String(), true)
}

func TestGZTarFileRetrieve(test *testing.T) {
	src, err := ioutil.TempFile("", "")
	if err != nil {
		test.Errorf("Impossible to create a temp file %v", err)
	}
	test.Logf("File: %v", src.Name())
	// defer os.Remove(src.Name())

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
			test.Errorf("Impossible to write TAR header: %v", err)
			return
		}
		if _, err = tw.Write([]byte(file.Body)); err != nil {
			test.Errorf("Impossible to write file %q to TAR: %v", file.Name, err)
			return
		}
	}
	// Make sure to check the error on Close
	if err = tw.Close(); err != nil {
		test.Errorf("Error closing TAR GZ file: %v", err)
	}
	gw.Close()
	src.Close()

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

// Helper function to use for different backends
func testRetrieve(test *testing.T, rawurl string, shouldFail bool) {
	t, err := CreateTask("cmd", rawurl)
	if err != nil {
		test.Errorf("Impossible to create a task: %v", err)
		return
	}
	defer t.Close()
	err = t.Retrieve()
	if shouldFail && err == nil {
		test.Errorf("It should fail on retrieving the task")
		return
	}
	if !shouldFail {
		if err != nil {
			test.Errorf("Error retrieving a task: %v", err)
			return
		}
		if t.ImagePath() == "" {
			test.Errorf("Not possible to get the image path")
			return
		}
		test.Logf("Image path: %q", t.ImagePath())
	}
}
