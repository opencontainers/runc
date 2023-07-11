/*
 * Copyright (c) 2023 SUSE LLC
 * Copyright (c) 2023 Aleksa Sarai <cyphar@cyphar.com>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/opencontainers/runc/libcontainer/dmz"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

// version will be populated by the Makefile, read from
// VERSION file of the source code.
var version = ""

// gitCommit will be the hash that the binary was built from
// and will be populated by the Makefile.
var gitCommit = ""

const (
	usage = `Open Container Initiative contrib/cmd/memfd-bind

In order to protect against certain container attacks, every runc invocation
that involves creating or joining a container will cause runc to make a copy of
the runc binary in memory (usually to a memfd). While "runc init" is very
short-lived, this extra memory usage can cause problems for containers with
very small memory limits (or containers that have many "runc exec" invocations
applied to them at the same time).

memfd-bind is a tool to create a persistent memfd-sealed-copy of the runc binary,
which will cause runc to not make its own copy. This means you can get the
benefits of using a sealed memfd as runc's binary (even in a container breakout
attack to get write access to the runc binary, neither the underlying binary
nor the memfd copy can be changed).

To use memfd-bind, just specify which path you want to create a socket path at
which you want to receive terminals:

    $ sudo memfd-bind /usr/bin/runc

Note that (due to kernel restrictions on bind-mounts), this program must remain
running on the host in order for the binary to be readable (it is recommended
you use a systemd unit to keep this process around).

If this program dies, there will be a leftover mountpoint that always returns
-EINVAL when attempting to access it. You need to use memfd-bind --cleanup on the
path in order to unmount the path (regular umount(8) will not work):

    $ sudo memfd-bind --cleanup /usr/bin/runc

Note that (due to restrictions on /proc/$pid/fd/$fd magic-link resolution),
only privileged users (specifically, those that have ptrace privileges over the
memfd-bind daemon) can access the memfd bind-mount. This means that using this
tool to harden your /usr/bin/runc binary would result in unprivileged users
being unable to execute the binary. If this is an issue, you could make all
privileged process use a different copy of runc (by making a copy in somewhere
like /usr/sbin/runc) and only using memfd-bind for the version used by
privileged users.
`
)

func cleanup(path string) error {
	file, err := os.OpenFile(path, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("cleanup: failed to open runc binary path: %w", err)
	}
	defer file.Close()
	fdPath := fmt.Sprintf("/proc/self/fd/%d", file.Fd())

	// Keep umounting until we hit a umount error.
	for unix.Unmount(fdPath, unix.MNT_DETACH) == nil {
		// loop...
		logrus.Debugf("memfd-bind: path %q unmount succeeded...", path)
	}
	logrus.Infof("memfd-bind: path %q has been cleared of all old bind-mounts", path)
	return nil
}

// memfdClone is a memfd-only implementation of dmz.CloneBinary.
func memfdClone(path string) (*os.File, error) {
	binFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open runc binary path: %w", err)
	}
	defer binFile.Close()
	stat, err := binFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("checking %s size: %w", path, err)
	}
	size := stat.Size()
	memfd, sealFn, err := dmz.Memfd("/proc/self/exe")
	if err != nil {
		return nil, fmt.Errorf("creating memfd failed: %w", err)
	}
	copied, err := io.Copy(memfd, binFile)
	if err != nil {
		return nil, fmt.Errorf("copy binary: %w", err)
	} else if copied != size {
		return nil, fmt.Errorf("copied binary size mismatch: %d != %d", copied, size)
	}
	if err := sealFn(&memfd); err != nil {
		return nil, fmt.Errorf("could not seal fd: %w", err)
	}
	if !dmz.IsCloned(memfd) {
		return nil, fmt.Errorf("cloned memfd is not properly sealed")
	}
	return memfd, nil
}

func mount(path string) error {
	memfdFile, err := memfdClone(path)
	if err != nil {
		return fmt.Errorf("memfd clone: %w", err)
	}
	defer memfdFile.Close()
	memfdPath := fmt.Sprintf("/proc/self/fd/%d", memfdFile.Fd())

	// We have to open an O_NOFOLLOW|O_PATH to the memfd magic-link because we
	// cannot bind-mount the memfd itself (it's in the internal kernel mount
	// namespace and cross-mount-namespace bind-mounts are not allowed). This
	// also requires that this program stay alive continuously for the
	// magic-link to stay alive...
	memfdLink, err := os.OpenFile(memfdPath, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("mount: failed to /proc/self/fd magic-link for memfd: %w", err)
	}
	defer memfdLink.Close()
	memfdLinkFdPath := fmt.Sprintf("/proc/self/fd/%d", memfdLink.Fd())

	exeFile, err := os.OpenFile(path, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("mount: failed to open target runc binary path: %w", err)
	}
	defer exeFile.Close()
	exeFdPath := fmt.Sprintf("/proc/self/fd/%d", exeFile.Fd())

	err = unix.Mount(memfdLinkFdPath, exeFdPath, "", unix.MS_BIND, "")
	if err != nil {
		return fmt.Errorf("mount: failed to mount memfd on top of runc binary path target: %w", err)
	}

	// If there is a signal we want to do cleanup.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, unix.SIGTERM, unix.SIGINT)
	go func() {
		<-sigCh
		logrus.Infof("memfd-bind: exit signal caught! cleaning up the bind-mount on %q...", path)
		_ = cleanup(path)
		os.Exit(0)
	}()

	// Clean up things we don't need...
	_ = exeFile.Close()
	_ = memfdLink.Close()

	// We now have to stay alive to keep the magic-link alive...
	logrus.Infof("memfd-bind: bind-mount of memfd over %q created -- looping forever!", path)
	for {
		// loop forever...
		time.Sleep(time.Duration(1<<63 - 1))
		// make sure the memfd isn't gc'd
		runtime.KeepAlive(memfdFile)
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "memfd-bind"
	app.Usage = usage

	// Set version to be the same as runC.
	var v []string
	if version != "" {
		v = append(v, version)
	}
	if gitCommit != "" {
		v = append(v, "commit: "+gitCommit)
	}
	app.Version = strings.Join(v, "\n")

	// Set the flags.
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "cleanup",
			Usage: "Do not create a new memfd-sealed file, only clean up an existing one at <path>.",
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Enable debug logging.",
		},
	}

	app.Action = func(ctx *cli.Context) error {
		args := ctx.Args()
		if len(args) != 1 {
			return errors.New("need to specify a single path to the runc binary")
		}
		path := ctx.Args()[0]

		if ctx.Bool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}

		err := cleanup(path)
		// We only care about cleanup errors when doing --cleanup.
		if ctx.Bool("cleanup") {
			return err
		}
		return mount(path)
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "memfd-bind: %v\n", err)
		os.Exit(1)
	}
}
