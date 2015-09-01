## Mount Configuration

Additional filesystems can be declared as "mounts", specified in the *mounts* object.
Keys in this object are names of mount points from portable config.
Values are objects with configuration of mount points.
The parameters are similar to the ones in [the Linux mount system call](http://man7.org/linux/man-pages/man2/mount.2.html).
Only [mounts from the portable config](config.md#mount-points) will be mounted.

* **type** (string, required) Linux, *filesystemtype* argument supported by the kernel are listed in */proc/filesystems* (e.g., "minix", "ext2", "ext3", "jfs", "xfs", "reiserfs", "msdos", "proc", "nfs", "iso9660"). Windows: ntfs
* **source** (string, required) a device name, but can also be a directory name or a dummy. Windows, the volume name that is the target of the mount point. \\?\Volume\{GUID}\ (on Windows source is called target)
* **options** (list of strings, optional) in the fstab format [https://wiki.archlinux.org/index.php/Fstab](https://wiki.archlinux.org/index.php/Fstab).

*Example (Linux)*

```json
"mounts": {
    "proc": {
        "type": "proc",
        "source": "proc",
        "options": []
    },
    "dev": {
        "type": "tmpfs",
        "source": "tmpfs",
        "options": ["nosuid","strictatime","mode=755","size=65536k"]
    },
    "devpts": {
        "type": "devpts",
        "source": "devpts",
        "options": ["nosuid","noexec","newinstance","ptmxmode=0666","mode=0620","gid=5"]
    },
    "data": {
        "type": "bind",
        "source": "/volumes/testing",
        "options": ["rbind","rw"]
    }
}
```

*Example (Windows)*

```json
"mounts": {
    "myfancymountpoint": {
        "type": "ntfs",
        "source": "\\\\?\\Volume\\{2eca078d-5cbc-43d3-aff8-7e8511f60d0e}\\",
        "options": []
    }
}
```

See links for details about [mountvol](http://ss64.com/nt/mountvol.html) and [SetVolumeMountPoint](https://msdn.microsoft.com/en-us/library/windows/desktop/aa365561(v=vs.85).aspx) in Windows.
