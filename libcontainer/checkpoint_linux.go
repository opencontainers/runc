// +build linux

package libcontainer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/golang/protobuf/proto"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/criurpc"
)

// check Criu version greater than or equal to min_version
func (c *linuxContainer) checkCriuVersion(min_version string) error {
	var x, y, z, versionReq int

	_, err := fmt.Sscanf(min_version, "%d.%d.%d\n", &x, &y, &z) // 1.5.2
	if err != nil {
		_, err = fmt.Sscanf(min_version, "Version: %d.%d\n", &x, &y) // 1.6
	}
	versionReq = x*10000 + y*100 + z

	out, err := exec.Command("criu", "-V").Output()
	if err != nil {
		return fmt.Errorf("Unable to execute CRIU command")
	}

	x = 0
	y = 0
	z = 0
	if ep := strings.Index(string(out), "-"); ep >= 0 {
		// criu Git version format
		var version string
		if sp := strings.Index(string(out), "GitID"); sp > 0 {
			version = string(out)[sp:ep]
		} else {
			return fmt.Errorf("Unable to parse the CRIU version")
		}

		n, err := fmt.Sscanf(string(version), "GitID: v%d.%d.%d", &x, &y, &z) // 1.5.2
		if err != nil {
			n, err = fmt.Sscanf(string(version), "GitID: v%d.%d", &x, &y) // 1.6
			y++
		} else {
			z++
		}
		if n < 2 || err != nil {
			return fmt.Errorf("Unable to parse the CRIU version: %s %d %s", version, n, err)
		}
	} else {
		// criu release version format
		n, err := fmt.Sscanf(string(out), "Version: %d.%d.%d\n", &x, &y, &z) // 1.5.2
		if err != nil {
			n, err = fmt.Sscanf(string(out), "Version: %d.%d\n", &x, &y) // 1.6
		}
		if n < 2 || err != nil {
			return fmt.Errorf("Unable to parse the CRIU version: %s %d %s", out, n, err)
		}
	}

	criuVersion := x*10000 + y*100 + z

	if criuVersion < versionReq {
		return fmt.Errorf("CRIU version must be %s or higher", min_version)
	}

	return nil
}

func (c *linuxContainer) addCriuDumpMount(req *criurpc.CriuReq, m *configs.Mount) {
	mountDest := m.Destination
	if strings.HasPrefix(mountDest, c.config.Rootfs) {
		mountDest = mountDest[len(c.config.Rootfs):]
	}

	extMnt := &criurpc.ExtMountMap{
		Key: proto.String(mountDest),
		Val: proto.String(mountDest),
	}
	req.Opts.ExtMnt = append(req.Opts.ExtMnt, extMnt)
}

func (c *linuxContainer) Checkpoint(opts *CheckpointOpts) error {
	c.m.Lock()
	defer c.m.Unlock()

	if err := c.checkCriuVersion("1.5.2"); err != nil {
		return err
	}

	if opts.ImagesDirectory == "" {
		return fmt.Errorf("invalid directory to save checkpoint")
	}

	// Since a container can be C/R'ed multiple times,
	// the checkpoint directory may already exist.
	if err := os.Mkdir(opts.ImagesDirectory, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	if opts.WorkDirectory == "" {
		opts.WorkDirectory = filepath.Join(c.root, "criu.work")
	}

	if err := os.Mkdir(opts.WorkDirectory, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	workDir, err := os.Open(opts.WorkDirectory)
	if err != nil {
		return err
	}
	defer workDir.Close()

	imageDir, err := os.Open(opts.ImagesDirectory)
	if err != nil {
		return err
	}
	defer imageDir.Close()

	rpcOpts := criurpc.CriuOpts{
		ImagesDirFd:    proto.Int32(int32(imageDir.Fd())),
		WorkDirFd:      proto.Int32(int32(workDir.Fd())),
		LogLevel:       proto.Int32(4),
		LogFile:        proto.String("dump.log"),
		Root:           proto.String(c.config.Rootfs),
		ManageCgroups:  proto.Bool(true),
		NotifyScripts:  proto.Bool(true),
		Pid:            proto.Int32(int32(c.initProcess.pid())),
		ShellJob:       proto.Bool(opts.ShellJob),
		LeaveRunning:   proto.Bool(opts.LeaveRunning),
		TcpEstablished: proto.Bool(opts.TcpEstablished),
		ExtUnixSk:      proto.Bool(opts.ExternalUnixConnections),
		FileLocks:      proto.Bool(opts.FileLocks),
		EmptyNs:        proto.Uint32(opts.EmptyNs),
	}

	// append optional criu opts, e.g., page-server and port
	if opts.PageServer.Address != "" && opts.PageServer.Port != 0 {
		rpcOpts.Ps = &criurpc.CriuPageServerInfo{
			Address: proto.String(opts.PageServer.Address),
			Port:    proto.Int32(opts.PageServer.Port),
		}
	}

	// append optional manage cgroups mode
	if opts.ManageCgroupsMode != 0 {
		if err := c.checkCriuVersion("1.7"); err != nil {
			return err
		}
		mode := criurpc.CriuCgMode(opts.ManageCgroupsMode)
		rpcOpts.ManageCgroupsMode = &mode
	}

	t := criurpc.CriuReqType_DUMP
	req := &criurpc.CriuReq{
		Type: &t,
		Opts: &rpcOpts,
	}

	for _, m := range c.config.Mounts {
		switch m.Device {
		case "bind":
			c.addCriuDumpMount(req, m)
			break
		case "cgroup":
			binds, err := getCgroupMounts(m)
			if err != nil {
				return err
			}
			for _, b := range binds {
				c.addCriuDumpMount(req, b)
			}
			break
		}
	}

	// Write the FD info to a file in the image directory

	fdsJSON, err := json.Marshal(c.initProcess.externalDescriptors())
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(opts.ImagesDirectory, descriptorsFilename), fdsJSON, 0655)
	if err != nil {
		return err
	}

	err = c.criuSwrk(nil, req, opts, false)
	if err != nil {
		return err
	}
	return nil
}

func (c *linuxContainer) addCriuRestoreMount(req *criurpc.CriuReq, m *configs.Mount) {
	mountDest := m.Destination
	if strings.HasPrefix(mountDest, c.config.Rootfs) {
		mountDest = mountDest[len(c.config.Rootfs):]
	}

	extMnt := &criurpc.ExtMountMap{
		Key: proto.String(mountDest),
		Val: proto.String(m.Source),
	}
	req.Opts.ExtMnt = append(req.Opts.ExtMnt, extMnt)
}

func (c *linuxContainer) restoreNetwork(req *criurpc.CriuReq, opts *CheckpointOpts) {
	for _, iface := range c.config.Networks {
		switch iface.Type {
		case "veth":
			veth := new(criurpc.CriuVethPair)
			veth.IfOut = proto.String(iface.HostInterfaceName)
			veth.IfIn = proto.String(iface.Name)
			req.Opts.Veths = append(req.Opts.Veths, veth)
			break
		case "loopback":
			break
		}
	}
	for _, i := range opts.VethPairs {
		veth := new(criurpc.CriuVethPair)
		veth.IfOut = proto.String(i.HostInterfaceName)
		veth.IfIn = proto.String(i.ContainerInterfaceName)
		req.Opts.Veths = append(req.Opts.Veths, veth)
	}
}

func (c *linuxContainer) Restore(process *Process, opts *CheckpointOpts) error {
	c.m.Lock()
	defer c.m.Unlock()
	if err := c.checkCriuVersion("1.5.2"); err != nil {
		return err
	}
	if opts.WorkDirectory == "" {
		opts.WorkDirectory = filepath.Join(c.root, "criu.work")
	}
	// Since a container can be C/R'ed multiple times,
	// the work directory may already exist.
	if err := os.Mkdir(opts.WorkDirectory, 0655); err != nil && !os.IsExist(err) {
		return err
	}
	workDir, err := os.Open(opts.WorkDirectory)
	if err != nil {
		return err
	}
	defer workDir.Close()
	if opts.ImagesDirectory == "" {
		return fmt.Errorf("invalid directory to restore checkpoint")
	}
	imageDir, err := os.Open(opts.ImagesDirectory)
	if err != nil {
		return err
	}
	defer imageDir.Close()
	// CRIU has a few requirements for a root directory:
	// * it must be a mount point
	// * its parent must not be overmounted
	// c.config.Rootfs is bind-mounted to a temporary directory
	// to satisfy these requirements.
	root := filepath.Join(c.root, "criu-root")
	if err := os.Mkdir(root, 0755); err != nil {
		return err
	}
	defer os.Remove(root)
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	err = syscall.Mount(c.config.Rootfs, root, "", syscall.MS_BIND|syscall.MS_REC, "")
	if err != nil {
		return err
	}
	defer syscall.Unmount(root, syscall.MNT_DETACH)
	t := criurpc.CriuReqType_RESTORE
	req := &criurpc.CriuReq{
		Type: &t,
		Opts: &criurpc.CriuOpts{
			ImagesDirFd:    proto.Int32(int32(imageDir.Fd())),
			WorkDirFd:      proto.Int32(int32(workDir.Fd())),
			EvasiveDevices: proto.Bool(true),
			LogLevel:       proto.Int32(4),
			LogFile:        proto.String("restore.log"),
			RstSibling:     proto.Bool(true),
			Root:           proto.String(root),
			ManageCgroups:  proto.Bool(true),
			NotifyScripts:  proto.Bool(true),
			ShellJob:       proto.Bool(opts.ShellJob),
			ExtUnixSk:      proto.Bool(opts.ExternalUnixConnections),
			TcpEstablished: proto.Bool(opts.TcpEstablished),
			FileLocks:      proto.Bool(opts.FileLocks),
			EmptyNs:        proto.Uint32(opts.EmptyNs),
		},
	}

	for _, m := range c.config.Mounts {
		switch m.Device {
		case "bind":
			c.addCriuRestoreMount(req, m)
			break
		case "cgroup":
			binds, err := getCgroupMounts(m)
			if err != nil {
				return err
			}
			for _, b := range binds {
				c.addCriuRestoreMount(req, b)
			}
			break
		}
	}

	if opts.EmptyNs&syscall.CLONE_NEWNET == 0 {
		c.restoreNetwork(req, opts)
	}

	// append optional manage cgroups mode
	if opts.ManageCgroupsMode != 0 {
		if err := c.checkCriuVersion("1.7"); err != nil {
			return err
		}
		mode := criurpc.CriuCgMode(opts.ManageCgroupsMode)
		req.Opts.ManageCgroupsMode = &mode
	}

	var (
		fds    []string
		fdJSON []byte
	)
	if fdJSON, err = ioutil.ReadFile(filepath.Join(opts.ImagesDirectory, descriptorsFilename)); err != nil {
		return err
	}

	if err := json.Unmarshal(fdJSON, &fds); err != nil {
		return err
	}
	for i := range fds {
		if s := fds[i]; strings.Contains(s, "pipe:") {
			inheritFd := new(criurpc.InheritFd)
			inheritFd.Key = proto.String(s)
			inheritFd.Fd = proto.Int32(int32(i))
			req.Opts.InheritFd = append(req.Opts.InheritFd, inheritFd)
		}
	}
	return c.criuSwrk(process, req, opts, true)
}

func (c *linuxContainer) criuApplyCgroups(pid int, req *criurpc.CriuReq) error {
	if err := c.cgroupManager.Apply(pid); err != nil {
		return err
	}

	path := fmt.Sprintf("/proc/%d/cgroup", pid)
	cgroupsPaths, err := cgroups.ParseCgroupFile(path)
	if err != nil {
		return err
	}

	for c, p := range cgroupsPaths {
		cgroupRoot := &criurpc.CgroupRoot{
			Ctrl: proto.String(c),
			Path: proto.String(p),
		}
		req.Opts.CgRoot = append(req.Opts.CgRoot, cgroupRoot)
	}

	return nil
}

func (c *linuxContainer) criuSwrk(process *Process, req *criurpc.CriuReq, opts *CheckpointOpts, applyCgroups bool) error {
	fds, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_SEQPACKET|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return err
	}

	logPath := filepath.Join(opts.WorkDirectory, req.GetOpts().GetLogFile())
	criuClient := os.NewFile(uintptr(fds[0]), "criu-transport-client")
	criuServer := os.NewFile(uintptr(fds[1]), "criu-transport-server")
	defer criuClient.Close()
	defer criuServer.Close()

	args := []string{"swrk", "3"}
	logrus.Debugf("Using CRIU with following args: %s", args)
	cmd := exec.Command("criu", args...)
	if process != nil {
		cmd.Stdin = process.Stdin
		cmd.Stdout = process.Stdout
		cmd.Stderr = process.Stderr
	}
	cmd.ExtraFiles = append(cmd.ExtraFiles, criuServer)

	if err := cmd.Start(); err != nil {
		return err
	}
	criuServer.Close()

	defer func() {
		criuClient.Close()
		_, err := cmd.Process.Wait()
		if err != nil {
			return
		}
	}()

	if applyCgroups {
		err := c.criuApplyCgroups(cmd.Process.Pid, req)
		if err != nil {
			return err
		}
	}

	var extFds []string
	if process != nil {
		extFds, err = getPipeFds(cmd.Process.Pid)
		if err != nil {
			return err
		}
	}

	logrus.Debugf("Using CRIU in %s mode", req.GetType().String())
	val := reflect.ValueOf(req.GetOpts())
	v := reflect.Indirect(val)
	for i := 0; i < v.NumField(); i++ {
		st := v.Type()
		name := st.Field(i).Name
		if strings.HasPrefix(name, "XXX_") {
			continue
		}
		value := val.MethodByName("Get" + name).Call([]reflect.Value{})
		logrus.Debugf("CRIU option %s with value %v", name, value[0])
	}
	data, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	_, err = criuClient.Write(data)
	if err != nil {
		return err
	}

	buf := make([]byte, 10*4096)
	for true {
		n, err := criuClient.Read(buf)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("unexpected EOF")
		}
		if n == len(buf) {
			return fmt.Errorf("buffer is too small")
		}

		resp := new(criurpc.CriuResp)
		err = proto.Unmarshal(buf[:n], resp)
		if err != nil {
			return err
		}
		if !resp.GetSuccess() {
			typeString := req.GetType().String()
			return fmt.Errorf("criu failed: type %s errno %d\nlog file: %s", typeString, resp.GetCrErrno(), logPath)
		}

		t := resp.GetType()
		switch {
		case t == criurpc.CriuReqType_NOTIFY:
			if err := c.criuNotifications(resp, process, opts, extFds); err != nil {
				return err
			}
			t = criurpc.CriuReqType_NOTIFY
			req = &criurpc.CriuReq{
				Type:          &t,
				NotifySuccess: proto.Bool(true),
			}
			data, err = proto.Marshal(req)
			if err != nil {
				return err
			}
			_, err = criuClient.Write(data)
			if err != nil {
				return err
			}
			continue
		case t == criurpc.CriuReqType_RESTORE:
		case t == criurpc.CriuReqType_DUMP:
			break
		default:
			return fmt.Errorf("unable to parse the response %s", resp.String())
		}

		break
	}

	// cmd.Wait() waits cmd.goroutines which are used for proxying file descriptors.
	// Here we want to wait only the CRIU process.
	st, err := cmd.Process.Wait()
	if err != nil {
		return err
	}
	if !st.Success() {
		return fmt.Errorf("criu failed: %s\nlog file: %s", st.String(), logPath)
	}
	return nil
}

// block any external network activity
func lockNetwork(config *configs.Config) error {
	for _, config := range config.Networks {
		strategy, err := getStrategy(config.Type)
		if err != nil {
			return err
		}

		if err := strategy.detach(config); err != nil {
			return err
		}
	}
	return nil
}

func unlockNetwork(config *configs.Config) error {
	for _, config := range config.Networks {
		strategy, err := getStrategy(config.Type)
		if err != nil {
			return err
		}
		if err = strategy.attach(config); err != nil {
			return err
		}
	}
	return nil
}

func (c *linuxContainer) criuNotifications(resp *criurpc.CriuResp, process *Process, opts *CheckpointOpts, fds []string) error {
	notify := resp.GetNotify()
	if notify == nil {
		return fmt.Errorf("invalid response: %s", resp.String())
	}
	switch {
	case notify.GetScript() == "post-dump":
		f, err := os.Create(filepath.Join(c.root, "checkpoint"))
		if err != nil {
			return err
		}
		f.Close()
	case notify.GetScript() == "network-unlock":
		if err := unlockNetwork(c.config); err != nil {
			return err
		}
	case notify.GetScript() == "network-lock":
		if err := lockNetwork(c.config); err != nil {
			return err
		}
	case notify.GetScript() == "setup-namespaces":
		if c.config.Hooks != nil {
			s := configs.HookState{
				Version: c.config.Version,
				ID:      c.id,
				Pid:     int(notify.GetPid()),
				Root:    c.config.Rootfs,
			}
			for _, hook := range c.config.Hooks.Prestart {
				if err := hook.Run(s); err != nil {
					return newSystemError(err)
				}
			}
		}
	case notify.GetScript() == "post-restore":
		pid := notify.GetPid()
		r, err := newRestoredProcess(int(pid), fds)
		if err != nil {
			return err
		}
		process.ops = r
		if err := c.state.transition(&restoredState{
			imageDir: opts.ImagesDirectory,
			c:        c,
		}); err != nil {
			return err
		}
		if err := c.updateState(r); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(c.root, "checkpoint")); err != nil {
			if !os.IsNotExist(err) {
				logrus.Error(err)
			}
		}
	}
	return nil
}
