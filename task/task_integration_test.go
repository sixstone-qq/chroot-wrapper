package task

// Integration tests for chroot-wrapper using task library. Basically,
// we want to run the binary itself to run commands in jail without a
// privileged user which requires to call itself as a new process

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// chroot-wrapper binary
var (
	chrootWrapperBinary = "chroot-wrapper"
	testImage           = flag.String("test-image", "", "Test image to retrieve")
)

func init() {
	if wrapperBin := os.Getenv("CHROOT_WRAPPER_BINARY"); len(wrapperBin) > 0 {
		chrootWrapperBinary = wrapperBin
	}

	var err error
	if chrootWrapperBinary, err = exec.LookPath(chrootWrapperBinary); err != nil {
		fmt.Printf("ERROR: couldn't resolve full path for wrapper binary: %v\n", err)
		os.Exit(1)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestStartTask(t *testing.T) {
	if len(*testImage) == 0 {
		t.Skip("Test image not available. Use -test-image to set it")
	}
	cmd := exec.Command(chrootWrapperBinary, *testImage, "pwd")

	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Output: %s\n", out)
		t.Fatalf("Failed to run: %v", err)
	}
	if string(out) != "/\n" {
		t.Errorf("Expected out is / != %s", out)
	}
}
