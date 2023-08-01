package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s path_to_mount_set_attr", os.Args[0])
	}

	src := os.Args[1]
	treeFD, err := unix.OpenTree(unix.AT_FDCWD, src, uint(unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC|unix.AT_EMPTY_PATH))
	if err != nil {
		log.Fatalf("error calling open_tree %q: %v", src, err)
	}
	defer unix.Close(treeFD)

	cmd := exec.Command("sleep", "5")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: 65536, Size: 65536}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: 65536, Size: 65536}},
	}
	if err := cmd.Start(); err != nil {
		log.Fatalf("failed to run the helper binary: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	path := fmt.Sprintf("/proc/%d/ns/user", cmd.Process.Pid)
	var userNsFile *os.File
	if userNsFile, err = os.Open(path); err != nil {
		log.Fatalf("unable to get user ns file descriptor: %v", err)
		return
	}
	defer userNsFile.Close()

	attr := unix.MountAttr{
		Attr_set:  unix.MOUNT_ATTR_IDMAP,
		Userns_fd: uint64(userNsFile.Fd()),
	}
	if err := unix.MountSetattr(treeFD, "", unix.AT_EMPTY_PATH, &attr); err != nil {
		log.Fatalf("error calling mount_setattr: %v", err)
	}
}
