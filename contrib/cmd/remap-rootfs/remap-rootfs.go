package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/urfave/cli"

	"github.com/opencontainers/runtime-spec/specs-go"
)

const usage = `contrib/cmd/remap-rootfs

remap-rootfs is a helper tool to remap the root filesystem of a Open Container
Initiative bundle using user namespaces such that the file owners are remapped
from "host" mappings to the user namespace's mappings.

Effectively, this is a slightly more complicated 'chown -R', and is primarily
used within runc's integration tests to remap the test filesystem to match the
test user namespace. Note that calling remap-rootfs multiple times, or changing
the mapping and then calling remap-rootfs will likely produce incorrect results
because we do not "un-map" any pre-applied mappings from previous remap-rootfs
calls.

Note that the bundle is assumed to be produced by a trusted source, and thus
malicious configuration files will likely not be handled safely.

To use remap-rootfs, simply pass it the path to an OCI bundle (a directory
containing a config.json):

    $ sudo remap-rootfs ./bundle
`

func toHostID(mappings []specs.LinuxIDMapping, id uint32) (int, bool) {
	for _, m := range mappings {
		if m.ContainerID <= id && id < m.ContainerID+m.Size {
			return int(m.HostID + id), true
		}
	}
	return -1, false
}

type inodeID struct {
	Dev, Ino uint64
}

func toInodeID(st *syscall.Stat_t) inodeID {
	return inodeID{Dev: st.Dev, Ino: st.Ino}
}

func remapRootfs(root string, uidMap, gidMap []specs.LinuxIDMapping) error {
	seenInodes := make(map[inodeID]struct{})
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		mode := info.Mode()
		st := info.Sys().(*syscall.Stat_t)

		// Skip symlinks.
		if mode.Type() == os.ModeSymlink {
			return nil
		}
		// Skip hard-links to files we've already remapped.
		id := toInodeID(st)
		if _, seen := seenInodes[id]; seen {
			return nil
		}
		seenInodes[id] = struct{}{}

		// Calculate the new uid:gid.
		uid := st.Uid
		newUID, ok1 := toHostID(uidMap, uid)
		gid := st.Gid
		newGID, ok2 := toHostID(gidMap, gid)

		// Skip files that cannot be mapped.
		if !ok1 || !ok2 {
			niceName := path
			if relName, err := filepath.Rel(root, path); err == nil {
				niceName = "/" + relName
			}
			fmt.Printf("skipping file %s: cannot remap user %d:%d -> %d:%d\n", niceName, uid, gid, newUID, newGID)
			return nil
		}
		if err := os.Lchown(path, newUID, newGID); err != nil {
			return err
		}
		// Re-apply any setid bits that would be cleared due to chown(2).
		return os.Chmod(path, mode)
	})
}

func main() {
	app := cli.NewApp()
	app.Name = "remap-rootfs"
	app.Usage = usage

	app.Action = func(ctx *cli.Context) error {
		args := ctx.Args()
		if len(args) != 1 {
			return errors.New("exactly one bundle argument must be provided")
		}
		bundle := args[0]

		configFile, err := os.Open(filepath.Join(bundle, "config.json"))
		if err != nil {
			return err
		}
		defer configFile.Close()

		var spec specs.Spec
		if err := json.NewDecoder(configFile).Decode(&spec); err != nil {
			return fmt.Errorf("parsing config.json: %w", err)
		}

		if spec.Root == nil {
			return errors.New("invalid config.json: root section is null")
		}
		rootfs := filepath.Join(bundle, spec.Root.Path)

		if spec.Linux == nil {
			return errors.New("invalid config.json: linux section is null")
		}
		uidMap := spec.Linux.UIDMappings
		gidMap := spec.Linux.GIDMappings
		if len(uidMap) == 0 && len(gidMap) == 0 {
			fmt.Println("skipping remapping -- no userns mappings specified")
			return nil
		}

		return remapRootfs(rootfs, uidMap, gidMap)
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
