package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage:", os.Args[0], "path_to_mount_set_attr")
		os.Exit(1)
	}
	src := os.Args[1]
	if err := supportsIDMap(src); err != nil {
		fmt.Fprintln(os.Stderr, "fatal error:", err)
		os.Exit(1)
	}
}

func supportsIDMap(src string) error {
	treeFD, err := unix.OpenTree(unix.AT_FDCWD, src, uint(unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC|unix.AT_EMPTY_PATH))
	if err != nil {
		return fmt.Errorf("error calling open_tree %q: %w", src, err)
	}
	defer unix.Close(treeFD)

	cmd := exec.Command("sleep", "5")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: 65536, Size: 65536}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: 65536, Size: 65536}},
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to run the helper binary: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	path := fmt.Sprintf("/proc/%d/ns/user", cmd.Process.Pid)
	var userNsFile *os.File
	if userNsFile, err = os.Open(path); err != nil {
		return fmt.Errorf("unable to get user ns file descriptor: %w", err)
	}
	defer userNsFile.Close()

	attr := unix.MountAttr{
		Attr_set:  unix.MOUNT_ATTR_IDMAP,
		Userns_fd: uint64(userNsFile.Fd()),
	}
	if err := unix.MountSetattr(treeFD, "", unix.AT_EMPTY_PATH, &attr); err != nil {
		return fmt.Errorf("error calling mount_setattr: %w", err)
	}

	return nil
}
