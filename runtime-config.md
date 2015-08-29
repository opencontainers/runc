## Mount Configuration

Additional filesystems can be declared as "mounts", specified in the *mounts* array. The parameters are similar to the ones in Linux mount system call. [http://linux.die.net/man/2/mount](http://linux.die.net/man/2/mount)

* **type** (string, required) Linux, *filesystemtype* argument supported by the kernel are listed in */proc/filesystems* (e.g., "minix", "ext2", "ext3", "jfs", "xfs", "reiserfs", "msdos", "proc", "nfs", "iso9660"). Windows: ntfs
* **source** (string, required) a device name, but can also be a directory name or a dummy. Windows, the volume name that is the target of the mount point. \\?\Volume\{GUID}\ (on Windows source is called target)
* **destination** (string, required) where the source filesystem is mounted relative to the container rootfs.
* **options** (list of strings, optional) in the fstab format [https://wiki.archlinux.org/index.php/Fstab](https://wiki.archlinux.org/index.php/Fstab).

*Example (Linux)*

```json
"mounts": [
    {
        "type": "proc",
        "source": "proc",
        "destination": "/proc",
        "options": []
    },
    {
        "type": "tmpfs",
        "source": "tmpfs",
        "destination": "/dev",
        "options": ["nosuid","strictatime","mode=755","size=65536k"]
    },
    {
        "type": "devpts",
        "source": "devpts",
        "destination": "/dev/pts",
        "options": ["nosuid","noexec","newinstance","ptmxmode=0666","mode=0620","gid=5"]
    },
    {
        "type": "bind",
        "source": "/volumes/testing",
        "destination": "/data",
        "options": ["rbind","rw"]
    }
]
```

*Example (Windows)*

```json
"mounts": [
    {
        "type": "ntfs",
        "source": "\\\\?\\Volume\\{2eca078d-5cbc-43d3-aff8-7e8511f60d0e}\\",
        "destination": "C:\\Users\\crosbymichael\\My Fancy Mount Point\\",
        "options": []
    }
]
```

See links for details about [mountvol](http://ss64.com/nt/mountvol.html) and [SetVolumeMountPoint](https://msdn.microsoft.com/en-us/library/windows/desktop/aa365561(v=vs.85).aspx) in Windows.
