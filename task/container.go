// Reference: http://lk4d4.darth.io/posts/unpriv3/
package task

// Unpriviliged way to run a command inside a jail (chrooted) using
// mount namespaces from linux OS

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

type Container struct {
	Args []string
}

func (c *Container) Run() error {
	name, err := exec.LookPath(c.Args[0])
	if err != nil {
		return fmt.Errorf("LookPath: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Getwd: %v", err)
	}
	// Set up the container environment
	if err := pivotRoot(wd); err != nil {
		return fmt.Errorf("Pivot root: %v", err)
	}

	log.Println("Launching", name, c.Args[1:])
	return syscall.Exec(name, c.Args, os.Environ())
}

// Use of pivot_root (2) in Linux
func pivotRoot(root string) (err error) {
	// we need this to satisfy restriction:
	// "new_root and put_old must not be on the same filesystem as the current root"
	if err = syscall.Mount(root, root, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("Mount rootfs to itself error: %v", err)
	}
	// create rootfs/.pivot_root as path for old_root
	pivotDir := filepath.Join(root, ".pivot_root")
	if err = os.Mkdir(pivotDir, 0777); err != nil {
		return
	}
	defer func() {
		// remove temporary directory
		rerr := os.Remove(pivotDir)
		if err == nil {
			err = rerr
		}
	}()
	// pivot_root to rootfs, now old_root is mounted in rootfs/.pivot_root
	// mounts from it still can be seen in `mount`
	if err = syscall.PivotRoot(root, pivotDir); err != nil {
		return fmt.Errorf("pivot_root %v", err)
	}
	// change working directory to /
	// it is recommendation from man-page
	if err = syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / %v", err)
	}
	// path to pivot root now changed, update
	pivotDir = filepath.Join("/", ".pivot_root")
	// umount rootfs/.pivot_root(which is now /.pivot_root) with all submounts
	// now we have only mounts that we mounted ourselves in `mount`
	if err = syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir %v", err)
	}
	return
}
