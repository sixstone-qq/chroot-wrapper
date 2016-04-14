package task

// Integration tests for chroot-wrapper using task library. Basically,
// we want to run the binary itself to run commands in jail without a
// privileged user which requires to call itself as a new process

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
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

// Basic start task test
func TestStartTask(t *testing.T) {
	if len(*testImage) == 0 {
		t.Skip("Test image not available. Use -test-image to set it")
	}
	cmd := exec.Command(chrootWrapperBinary, "run", *testImage, "pwd")

	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Output: %s\n", out)
		t.Fatalf("Failed to run: %v", err)
	}
	if !strings.Contains(string(out), "/\n") {
		t.Errorf("Expected out is / != %s", out)
	}
}

// Test signal a task and status
// Start a task, stop it, start it and kill it
func TestKillTask(t *testing.T) {
	if len(*testImage) == 0 {
		t.Skip("Test image not available. Use -test-image to set it")
	}
	// Start it!
	cmd := exec.Command(chrootWrapperBinary, "-port", "8888", "run", *testImage,
		"sleep", "1000")

	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	// Wait a little
	time.Sleep(1 * time.Second)

	// Stop it!
	killCmd := exec.Command(chrootWrapperBinary, "-port", "8888", "kill", "SIGSTOP")
	out, err := killCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to stop the task: %v", err)
	}
	if !strings.Contains(string(out), "Signaled") {
		t.Errorf("The output from kill was not correct: %s", out)
	}

	statusCmd := exec.Command(chrootWrapperBinary, "-port", "8888", "ps")
	out, err = statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to stat the task: %v", err)
	}
	if !strings.Contains(string(out), "Stopped") {
		t.Errorf("Status %s different from Stopped", out)
	}

	// Resume it!
	killCmd = exec.Command(chrootWrapperBinary, "-port", "8888", "kill", "SIGCONT")
	out, err = killCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to continue the task: %v", err)
	}
	if !strings.Contains(string(out), "Signaled") {
		t.Errorf("The output from kill was not correct: %s", out)
	}

	statusCmd = exec.Command(chrootWrapperBinary, "-port", "8888", "ps")
	out, err = statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to stat the task: %v", err)
	}
	if !(strings.Contains(string(out), "Sleeping") ||
		!strings.Contains(string(out), "Running")) {
		t.Errorf("Status %s different from Sleeping|Running", out)
	}

	// Terminate it!
	killCmd = exec.Command(chrootWrapperBinary, "-port", "8888", "kill", "SIGTERM")
	out, err = killCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to terminate the task: %v", err)
	}
	if !strings.Contains(string(out), "Signaled") {
		t.Errorf("The output from kill was not correct: %s", out)
	}

	// Check the end was by a signal
	err = cmd.Wait()
	if err != nil {
		t.Errorf("Normal end instead of signaled")
	}
}

// Test working directory
func TestWDTask(t *testing.T) {
	if len(*testImage) == 0 {
		t.Skip("Test image not available. Use -test-image to set it")
	}
	cmd := exec.Command(chrootWrapperBinary, "-wd", "/bin", "run", *testImage, "pwd")

	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Output: %s\n", out)
		t.Fatalf("Failed to run: %v", err)
	}
	if !strings.Contains(string(out), "/bin\n") {
		t.Errorf("Expected out is /bin != %s", out)
	}
}
