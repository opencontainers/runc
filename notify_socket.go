package main

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

type notifySocket struct {
	socket     *net.UnixConn
	host       string
	socketPath string
}

func newNotifySocket(context *cli.Context, notifySocketHost string, id string) *notifySocket {
	if notifySocketHost == "" {
		return nil
	}

	root := filepath.Join(context.GlobalString("root"), id)
	socketPath := filepath.Join(root, "notify", "notify.sock")

	notifySocket := &notifySocket{
		socket:     nil,
		host:       notifySocketHost,
		socketPath: socketPath,
	}

	return notifySocket
}

func (s *notifySocket) Close() error {
	return s.socket.Close()
}

// If systemd is supporting sd_notify protocol, this function will add support
// for sd_notify protocol from within the container.
func (s *notifySocket) setupSpec(spec *specs.Spec) {
	pathInContainer := filepath.Join("/run/notify", path.Base(s.socketPath))
	mount := specs.Mount{
		Destination: path.Dir(pathInContainer),
		Source:      path.Dir(s.socketPath),
		Options:     []string{"bind", "nosuid", "noexec", "nodev", "ro"},
	}
	spec.Mounts = append(spec.Mounts, mount)
	spec.Process.Env = append(spec.Process.Env, "NOTIFY_SOCKET="+pathInContainer)
}

func (s *notifySocket) bindSocket() error {
	addr := net.UnixAddr{
		Name: s.socketPath,
		Net:  "unixgram",
	}

	socket, err := net.ListenUnixgram("unixgram", &addr)
	if err != nil {
		return err
	}

	err = os.Chmod(s.socketPath, 0o777)
	if err != nil {
		socket.Close()
		return err
	}

	s.socket = socket
	return nil
}

func (s *notifySocket) setupSocketDirectory() error {
	return os.Mkdir(path.Dir(s.socketPath), 0o755)
}

func notifySocketStart(context *cli.Context, notifySocketHost, id string) (*notifySocket, error) {
	notifySocket := newNotifySocket(context, notifySocketHost, id)
	if notifySocket == nil {
		return nil, nil
	}

	if err := notifySocket.bindSocket(); err != nil {
		return nil, err
	}
	return notifySocket, nil
}

func (s *notifySocket) waitForContainer(container *libcontainer.Container) error {
	state, err := container.State()
	if err != nil {
		return err
	}
	return s.run(state.InitProcessPid)
}

func (n *notifySocket) run(pid1 int) error {
	if n.socket == nil {
		return nil
	}
	notifySocketHostAddr := net.UnixAddr{Name: n.host, Net: "unixgram"}
	client, err := net.DialUnix("unixgram", nil, &notifySocketHostAddr)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()

	fileChan := make(chan []byte)
	go func() {
		for {
			buf := make([]byte, 4096)
			r, err := n.socket.Read(buf)
			if err != nil {
				return
			}
			got := buf[0:r]
			// systemd-ready sends a single datagram with the state string as payload,
			// so we don't need to worry about partial messages.
			for _, line := range bytes.Split(got, []byte{'\n'}) {
				if bytes.HasPrefix(got, []byte("READY=")) {
					fileChan <- line
					return
				}
			}

		}
	}()

	for {
		select {
		case <-ticker.C:
			_, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid1)))
			if err != nil {
				return nil
			}
		case b := <-fileChan:
			return notifyHost(client, b, pid1)
		}
	}
}

// notifyHost tells the host (usually systemd) that the container reported READY.
// Also sends MAINPID and BARRIER.
func notifyHost(client *net.UnixConn, ready []byte, pid1 int) error {
	_, err := client.Write(append(ready, '\n'))
	if err != nil {
		return err
	}

	// now we can inform systemd to use pid1 as the pid to monitor
	newPid := "MAINPID=" + strconv.Itoa(pid1)
	_, err = client.Write([]byte(newPid + "\n"))
	if err != nil {
		return err
	}

	// wait for systemd to acknowledge the communication
	return sdNotifyBarrier(client)
}

// errUnexpectedRead is reported when actual data was read from the pipe used
// to synchronize with systemd. Usually, that pipe is only closed.
var errUnexpectedRead = errors.New("unexpected read from synchronization pipe")

// sdNotifyBarrier performs synchronization with systemd by means of the sd_notify_barrier protocol.
func sdNotifyBarrier(client *net.UnixConn) error {
	// Create a pipe for communicating with systemd daemon.
	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return err
	}

	// Get the FD for the unix socket file to be able to do perform syscall.Sendmsg.
	clientFd, err := client.File()
	if err != nil {
		return err
	}

	// Send the write end of the pipe along with a BARRIER=1 message.
	fdRights := unix.UnixRights(int(pipeW.Fd()))
	err = unix.Sendmsg(int(clientFd.Fd()), []byte("BARRIER=1"), fdRights, nil, 0)
	if err != nil {
		return &os.SyscallError{Syscall: "sendmsg", Err: err}
	}

	// Close our copy of pipeW.
	err = pipeW.Close()
	if err != nil {
		return err
	}

	// Expect the read end of the pipe to be closed after 30 seconds.
	err = pipeR.SetReadDeadline(time.Now().Add(30 * time.Second))
	if err != nil {
		return nil
	}

	// Read a single byte expecting EOF.
	var buf [1]byte
	n, err := pipeR.Read(buf[:])
	if n != 0 || err == nil {
		return errUnexpectedRead
	} else if errors.Is(err, os.ErrDeadlineExceeded) {
		// Probably the other end doesn't support the sd_notify_barrier protocol.
		logrus.Warn("Timeout after waiting 30s for barrier. Ignored.")
		return nil
	} else if err == io.EOF { //nolint:errorlint // https://github.com/polyfloyd/go-errorlint/issues/49
		return nil
	} else {
		return err
	}
}
