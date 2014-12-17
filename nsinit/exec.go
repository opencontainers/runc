package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"github.com/codegangsta/cli"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/configs"
	consolepkg "github.com/docker/libcontainer/console"
	"github.com/docker/libcontainer/namespaces"
)

var (
	dataPath  = os.Getenv("data_path")
	console   = os.Getenv("console")
	rawPipeFd = os.Getenv("pipe")
)

var execCommand = cli.Command{
	Name:   "exec",
	Usage:  "execute a new command inside a container",
	Action: execAction,
	Flags: []cli.Flag{
		cli.BoolFlag{Name: "list", Usage: "list all registered exec functions"},
		cli.StringFlag{Name: "func", Value: "exec", Usage: "function name to exec inside a container"},
	},
}

func execAction(context *cli.Context) {
	if context.Bool("list") {
		w := tabwriter.NewWriter(os.Stdout, 10, 1, 3, ' ', 0)
		fmt.Fprint(w, "NAME\tUSAGE\n")

		for k, f := range argvs {
			fmt.Fprintf(w, "%s\t%s\n", k, f.Usage)
		}

		w.Flush()

		return
	}

	var exitCode int

	process := &libcontainer.ProcessConfig{
		Args:   context.Args(),
		Env:    context.StringSlice("env"),
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	factory, err := libcontainer.New(context.GlobalString("root"), []string{os.Args[0], "init", "--fd", "3", "--"})
	if err != nil {
		log.Fatal(err)
	}

	id := fmt.Sprintf("%x", md5.Sum([]byte(dataPath)))
	container, err := factory.Load(id)
	if err != nil && !os.IsNotExist(err) {
		var config *configs.Config

		config, err = loadConfig()
		if err != nil {
			log.Fatal(err)
		}
		container, err = factory.Create(id, config)
	}
	if err != nil {
		log.Fatal(err)
	}

	pid, err := container.StartProcess(process)
	if err != nil {
		log.Fatalf("failed to exec: %s", err)
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		log.Fatalf("Unable to find the %d process: %s", pid, err)
	}

	ps, err := p.Wait()
	if err != nil {
		log.Fatalf("Unable to wait the %d process: %s", pid, err)
	}
	container.Destroy()

	status := ps.Sys().(syscall.WaitStatus)
	if status.Exited() {
		exitCode = status.ExitStatus()
	} else if status.Signaled() {
		exitCode = -int(status.Signal())
	} else {
		log.Fatalf("Unexpected status")
	}

	os.Exit(exitCode)
}

// the process for execing a new process inside an existing container is that we have to exec ourself
// with the nsenter argument so that the C code can setns an the namespaces that we require.  Then that
// code path will drop us into the path that we can do the final setup of the namespace and exec the users
// application.
func startInExistingContainer(config *configs.Config, state *configs.State, action string, context *cli.Context) (int, error) {
	var (
		master  *os.File
		console string
		err     error

		sigc = make(chan os.Signal, 10)

		stdin  = os.Stdin
		stdout = os.Stdout
		stderr = os.Stderr
	)
	signal.Notify(sigc)

	if config.Tty {
		stdin = nil
		stdout = nil
		stderr = nil

		master, console, err = consolepkg.CreateMasterAndConsole()
		if err != nil {
			return -1, err
		}

		go io.Copy(master, os.Stdin)
		go io.Copy(os.Stdout, master)

		state, err := term.SetRawTerminal(os.Stdin.Fd())
		if err != nil {
			return -1, err
		}

		defer term.RestoreTerminal(os.Stdin.Fd(), state)
	}

	startCallback := func(cmd *exec.Cmd) {
		go func() {
			resizeTty(master)

			for sig := range sigc {
				switch sig {
				case syscall.SIGWINCH:
					resizeTty(master)
				default:
					cmd.Process.Signal(sig)
				}
			}
		}()
	}

	return namespaces.ExecIn(config, state, context.Args(), os.Args[0], action, stdin, stdout, stderr, console, startCallback)
}

// startContainer starts the container. Returns the exit status or -1 and an
// error.
//
// Signals sent to the current process will be forwarded to container.
func startContainer(container *configs.Config, dataPath string, args []string) (int, error) {
	var (
		cmd  *exec.Cmd
		sigc = make(chan os.Signal, 10)
	)

	signal.Notify(sigc)

	createCommand := func(container *configs.Config, console, dataPath, init string, pipe *os.File, args []string) *exec.Cmd {
		cmd = namespaces.DefaultCreateCommand(container, console, dataPath, init, pipe, args)
		if logPath != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("log=%s", logPath))
		}
		return cmd
	}

	var (
		master  *os.File
		console string
		err     error

		stdin  = os.Stdin
		stdout = os.Stdout
		stderr = os.Stderr
	)

	if container.Tty {
		stdin = nil
		stdout = nil
		stderr = nil

		master, console, err = consolepkg.CreateMasterAndConsole()
		if err != nil {
			return -1, err
		}

		go io.Copy(master, os.Stdin)
		go io.Copy(os.Stdout, master)

		state, err := term.SetRawTerminal(os.Stdin.Fd())
		if err != nil {
			return -1, err
		}

		defer term.RestoreTerminal(os.Stdin.Fd(), state)
	}

	startCallback := func() {
		go func() {
			resizeTty(master)

			for sig := range sigc {
				switch sig {
				case syscall.SIGWINCH:
					resizeTty(master)
				default:
					cmd.Process.Signal(sig)
				}
			}
		}()
	}

	return namespaces.Exec(container, stdin, stdout, stderr, console, dataPath, args, createCommand, startCallback)
}

func resizeTty(master *os.File) {
	if master == nil {
		return
	}

	ws, err := term.GetWinsize(os.Stdin.Fd())
	if err != nil {
		return
	}

	if err := term.SetWinsize(master.Fd(), ws); err != nil {
		return
	}
}
