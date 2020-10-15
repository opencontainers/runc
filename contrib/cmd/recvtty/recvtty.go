/*
 * Copyright 2016 SUSE LLC
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
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"github.com/containerd/console"
	"github.com/opencontainers/runc/libcontainer/utils"
	"github.com/urfave/cli"
)

// version will be populated by the Makefile, read from
// VERSION file of the source code.
var version = ""

// gitCommit will be the hash that the binary was built from
// and will be populated by the Makefile
var gitCommit = ""

const (
	usage = `Open Container Initiative contrib/cmd/recvtty

recvtty is a reference implementation of a consumer of runC's --console-socket
API. It has two main modes of operation:

  * single: Only permit one terminal to be sent to the socket, which is
	then hooked up to the stdio of the recvtty process. This is useful
	for rudimentary shell management of a container.

  * null: Permit as many terminals to be sent to the socket, but they
	are read to /dev/null. This is used for testing, and imitates the
	old runC API's --console=/dev/pts/ptmx hack which would allow for a
	similar trick. This is probably not what you want to use, unless
	you're doing something like our bats integration tests.

To use recvtty, just specify a socket path at which you want to receive
terminals:

    $ recvtty [--mode <single|null>] socket.sock
`
)

func bail(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "[recvtty] fatal error: %v\n", err)
	os.Exit(1)
}

func handleSingle(path string, noStdin bool) (retErr error) {
	// Open a socket.
	ln, retErr := net.Listen("unix", path)
	if retErr != nil {
		return retErr
	}
	defer func() {
		_ = ln.Close()
	}()

	// We only accept a single connection, since we can only really have
	// one reader for os.Stdin. Plus this is all a PoC.
	conn, retErr := ln.Accept()
	if retErr != nil {
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	// Close ln, to allow for other instances to take over.
	_ = ln.Close()

	// Get the fd of the connection.
	unixconn, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("failed to cast to unixconn")
	}

	socket, retErr := unixconn.File()
	if retErr != nil {
		return
	}
	defer func() {
		_ = socket.Close()
	}()

	// Get the master file descriptor from runC.
	master, retErr := utils.RecvFd(socket)
	if retErr != nil {
		return
	}
	c, retErr := console.ConsoleFromFile(master)
	if retErr != nil {
		return
	}
	if err := console.ClearONLCR(c.Fd()); err != nil {
		return err
	}

	// Copy from our stdio to the master fd.
	quitChan := make(chan struct{})
	errChan := make(chan error)
	go func() {
		_, err := io.Copy(os.Stdout, c)
		if err != nil {
			errChan <- err
		}
		quitChan <- struct{}{}
	}()
	if !noStdin {
		go func() {
			_, err := io.Copy(c, os.Stdin)
			if err != nil {
				errChan <- err
			}
			quitChan <- struct{}{}
		}()
	}

	// Only close the master fd once we've stopped copying.
	select {
	case retErr = <-errChan:
		return retErr
	case <-quitChan:
		retErr = c.Close()
		return retErr
	}
}

func handleNull(path string) (retErr error) {
	// Open a socket.
	ln, retErr := net.Listen("unix", path)
	if retErr != nil {
		return retErr
	}
	defer func() {
		_ = ln.Close()
	}()

	// As opposed to handleSingle we accept as many connections as we get, but
	// we don't interact with Stdin at all (and we copy stdout to /dev/null).
	for {
		conn, retErr := ln.Accept()
		if retErr != nil {
			return retErr
		}
		go func(conn net.Conn) {
			// Don't leave references lying around.
			defer func() {
				_ = conn.Close()
			}()

			// Get the fd of the connection.
			unixconn, ok := conn.(*net.UnixConn)
			if !ok {
				return
			}

			socket, retErr := unixconn.File()
			if retErr != nil {
				return
			}
			defer func() {
				_ = socket.Close()
			}()

			// Get the master file descriptor from runC.
			master, retErr := utils.RecvFd(socket)
			if retErr != nil {
				return
			}

			// Just do a dumb copy to /dev/null.
			devnull, retErr := os.OpenFile("/dev/null", os.O_RDWR, 0)
			if retErr != nil {
				// TODO: Handle this nicely.
				return
			}

			_, _ = io.Copy(devnull, master)
			_ = devnull.Close()
		}(conn)
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "recvtty"
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
		cli.StringFlag{
			Name:  "mode, m",
			Value: "single",
			Usage: "Mode of operation (single or null)",
		},
		cli.StringFlag{
			Name:  "pid-file",
			Value: "",
			Usage: "Path to write daemon process ID to",
		},
		cli.BoolFlag{
			Name:  "no-stdin",
			Usage: "Disable stdin handling (no-op for null mode)",
		},
	}

	app.Action = func(ctx *cli.Context) error {
		args := ctx.Args()
		if len(args) != 1 {
			return fmt.Errorf("need to specify a single socket path")
		}
		path := ctx.Args()[0]

		pidPath := ctx.String("pid-file")
		if pidPath != "" {
			pid := fmt.Sprintf("%d\n", os.Getpid())
			if err := ioutil.WriteFile(pidPath, []byte(pid), 0644); err != nil {
				return err
			}
		}

		noStdin := ctx.Bool("no-stdin")
		switch ctx.String("mode") {
		case "single":
			if err := handleSingle(path, noStdin); err != nil {
				return err
			}
		case "null":
			if err := handleNull(path); err != nil {
				return err
			}
		default:
			return fmt.Errorf("need to select a valid mode: %s", ctx.String("mode"))
		}
		return nil
	}
	if err := app.Run(os.Args); err != nil {
		bail(err)
	}
}
