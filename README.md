## runc

`runc` is a CLI tool for spawning and running containers according to the OCF specification.

## State of the project

Currently `runc` is an implementation of the OCF specification.  We are currently sprinting
to have a v1 of the spec out within a quick timeframe of a few weeks, ~July 2015, 
so the `runc` config format will be constantly changing until 
the spec is finalized.  However, we encourage you to try out the tool and give feedback.

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
`container.json` file in the current working directory.

```bash
runc 
/ $ ps
PID   USER     COMMAND
1     daemon   sh
5     daemon   sh
/ $ 
```

### OCF Container JSON Format:

Below is a sample `container.json` configuration file. It assumes that
the file-system is found in a directory called `rootfs` and there is a
user named `daemon` defined within that file-system.

```json
{
    "version": "0.1",
    "os": "linux",
    "arch": "amd64",
    "processes": [
        {
            "tty": true,
            "user": "daemon",
            "args": [
                "sh"
            ],
            "env": [
                "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
                "TERM=xterm"
            ],
            "cwd": ""
        }
    ],
    "root": {
        "path": "rootfs",
        "readonly": true
    },
    "cpus": 1.1,
    "memory": 1024,
    "hostname": "shell",
    "namespaces": [
        {
            "type": "process"
        },
        {
            "type": "network"
        },
        {
            "type": "mount"
        },
        {
            "type": "ipc"
        },
        {
            "type": "uts"
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
    ],
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
    ]
}
```

### Examples:

#### Using a Docker image

To test using Docker's `busybox` image follow these steps:
* Install `docker` and download the `buysbox` image: `docker pull busybox`
* Create a container from that image and export its contents to a tar file:
`docker export $(docker create busybox) > busybox.tar`
* Untar the contents to create your filesystem directory:
```
mkdir rootfs
tar -C rootfs -xf busybox.tar
```
* Create a file called `container.json` using the example from above.
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
