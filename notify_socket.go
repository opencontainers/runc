// +build linux

package main

import (
	"fmt"
	"net"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
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
	path := filepath.Join(root, "notify.sock")

	notifySocket := &notifySocket{
		socket:     nil,
		host:       notifySocketHost,
		socketPath: path,
	}

	return notifySocket
}

func (ns *notifySocket) Close() error {
	return ns.socket.Close()
}

// If systemd is supporting sd_notify protocol, this function will add support
// for sd_notify protocol from within the container.
func (s *notifySocket) setupSpec(context *cli.Context, spec *specs.Spec) {
	mount := specs.Mount{Destination: s.host, Type: "bind", Source: s.socketPath, Options: []string{"bind"}}
	spec.Mounts = append(spec.Mounts, mount)
	spec.Process.Env = append(spec.Process.Env, fmt.Sprintf("NOTIFY_SOCKET=%s", s.host))
}

func (s *notifySocket) setupSocket() error {
	addr := net.UnixAddr{
		Name: s.socketPath,
		Net:  "unixgram",
	}

	socket, err := net.ListenUnixgram("unixgram", &addr)
	if err != nil {
		return err
	}

	s.socket = socket
	return nil
}

func (notifySocket *notifySocket) run() {
	buf := make([]byte, 512)
	notifySocketHostAddr := net.UnixAddr{Name: notifySocket.host, Net: "unixgram"}
	client, err := net.DialUnix("unixgram", nil, &notifySocketHostAddr)
	if err != nil {
		logrus.Error(err)
		return
	}
	for {
		r, err := notifySocket.socket.Read(buf)
		if err != nil {
			break
		}

		client.Write(buf[0:r])
	}
}
