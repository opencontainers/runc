## runc

`runc` is a CLI tool for spawning and running containers according to the OCF specification.

## State of the project

Currently `runc` is an implementation of the OCF specification.  We are currently sprinting
to have a v1 of the spec out within a quick timeframe of a few weeks, ~July 2015,
so the `runc` config format will be constantly changing until
the spec is finalized.  However, we encourage you to try out the tool and give feedback.

### OCF

How does `runc` integrate with the Open Container Format?  `runc` depends on the types
specified in the [specs](https://github.com/opencontainers/specs) repository.  Whenever
the specification is updated and ready to be versioned `runc` will update it's dependency
on the specs repository and support the update spec.

### Building:

```bash
# create a 'github.com/opencontainers' in your GOPATH
cd github.com/opencontainers
git clone https://github.com/opencontainers/runc
cd runc
make
sudo make install
```

### Using:

To run a container that you received just execute `runc` with the JSON format as the argument or have a
`config.json` file in the current working directory.

```bash
runc
/ $ ps
PID   USER     COMMAND
1     daemon   sh
5     daemon   sh
/ $
```

### OCF Container JSON Format:

Below is a sample `config.json` configuration file. It assumes that
the file-system is found in a directory called `rootfs` and there is a
user named `daemon` defined within that file-system.

```json
{
    "version": "pre-draft",
    "platform": {
        "os": "linux",
        "arch": "amd64"
    },
    "process": {
        "terminal": true,
        "user": {
            "uid": 0,
            "gid": 0,
            "additionalGids": null
        },
        "args": [
            "sh"
        ],
        "env": [
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            "TERM=xterm"
        ],
        "cwd": ""
    },
    "root": {
        "path": "rootfs",
        "readonly": true
    },
    "hostname": "shell",
    "mounts": [
        {
            "type": "proc",
            "source": "proc",
            "destination": "/proc",
            "options": ""
        },
        {
            "type": "tmpfs",
            "source": "tmpfs",
            "destination": "/dev",
            "options": "nosuid,strictatime,mode=755,size=65536k"
        },
        {
            "type": "devpts",
            "source": "devpts",
            "destination": "/dev/pts",
            "options": "nosuid,noexec,newinstance,ptmxmode=0666,mode=0620,gid=5"
        },
        {
            "type": "tmpfs",
            "source": "shm",
            "destination": "/dev/shm",
            "options": "nosuid,noexec,nodev,mode=1777,size=65536k"
        },
        {
            "type": "mqueue",
            "source": "mqueue",
            "destination": "/dev/mqueue",
            "options": "nosuid,noexec,nodev"
        },
        {
            "type": "sysfs",
            "source": "sysfs",
            "destination": "/sys",
            "options": "nosuid,noexec,nodev"
        }
    ],
    "linux": {
        "uidMapping": null,
        "gidMapping": null,
        "rlimits": null,
        "systemProperties": null,
        "resources": {
            "disableOOMKiller": false,
            "memory": {
                "limit": 0,
                "reservation": 0,
                "swap": 0,
                "kernel": 0
            },
            "cpu": {
                "shares": 0,
                "quota": 0,
                "period": 0,
                "realtimeRuntime": 0,
                "realtimePeriod": 0,
                "cpus": "",
                "mems": ""
            },
            "blockIO": {
                "blkioWeight": 0,
                "blkioWeightDevice": "",
                "blkioThrottleReadBpsDevice": "",
                "blkioThrottleWriteBpsDevice": "",
                "blkioThrottleReadIopsDevice": "",
                "blkioThrottleWriteIopsDevice": ""
            },
            "hugepageLimits": null,
            "network": {
                "classId": "",
                "priorities": null
            }
        },
        "namespaces": [
            {
                "type": "process",
                "path": ""
            },
            {
                "type": "network",
                "path": ""
            },
            {
                "type": "ipc",
                "path": ""
            },
            {
                "type": "uts",
                "path": ""
            },
            {
                "type": "mount",
                "path": ""
            }
        ],
        "capabilities": [
            "AUDIT_WRITE",
            "KILL",
            "NET_BIND_SERVICE"
        ],
        "devices": [
            "null",
            "random",
            "full",
            "tty",
            "zero",
            "urandom"
        ]
    }
}
```

### Examples:

#### Using a Docker image (requires version 1.3 or later)

To test using Docker's `busybox` image follow these steps:
* Install `docker` and download the `busybox` image: `docker pull busybox`
* Create a container from that image and export its contents to a tar file:
`docker export $(docker create busybox) > busybox.tar`
* Untar the contents to create your filesystem directory:
```
mkdir rootfs
tar -C rootfs -xf busybox.tar
```
* Create a file called `config.json` using the example from above.
Modify the `user` property to be `root`.
* Execute `runc` and you should be placed into a shell where you can run `ps`:
```
$ runc
/ # ps
PID   USER     COMMAND
    1 root     sh
    9 root     ps
```

#### Using runc with systemd

```service
[Unit]
Description=Minecraft Build Server
Documentation=http://minecraft.net
After=network.target

[Service]
CPUQuota=200%
MemoryLimit=1536M
ExecStart=/usr/local/bin/runc
Restart=on-failure
WorkingDirectory=/containers/minecraftbuild

[Install]
WantedBy=multi-user.target
```
