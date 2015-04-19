package libcontainer

type CriuOpts struct {
	ImagesDirectory         string // directory for storing image files
	WorkDirectory           string // directory to cd and write logs/pidfiles/stats to
	LeaveRunning            bool   // leave container in running state after checkpoint
	TcpEstablished          bool   // checkpoint/restore established TCP connections
	ExternalUnixConnections bool   // allow external unix connections
	ShellJob                bool   // allow to dump and restore shell jobs
}
