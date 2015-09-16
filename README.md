## runc

`runc` is a CLI tool for spawning and running containers according to the OCF specification.

## State of the project

Currently `runc` is an implementation of the OCI specification.  We are currently sprinting
to have a v1 of the spec out within a quick timeframe of a few weeks, ~July 2015,
so the `runc` config format will be constantly changing until
the spec is finalized.  However, we encourage you to try out the tool and give feedback.

### OCF

How does `runc` integrate with the Open Container Initiative Specification?
`runc` depends on the types specified in the
[specs](https://github.com/opencontainers/specs) repository. Whenever the
specification is updated and ready to be versioned `runc` will update its dependency
on the specs repository and support the update spec.

### Building:

At the time of writing, runc only builds on the Linux platform.

```bash
# create a 'github.com/opencontainers' in your GOPATH/src
cd github.com/opencontainers
git clone https://github.com/opencontainers/runc
cd runc
make
sudo make install
```

In order to enable seccomp support you will need to install libseccomp on your platform.
If you do not with to build `runc` with seccomp support you can add `BUILDTAGS=""` when running make.

#### Build Tags

`runc` supports optional build tags for compiling in support for various features.


| Build Tag | Feature                            | Dependency  |
|-----------|------------------------------------|-------------|
| seccomp   | Syscall filtering                  | libseccomp  |
| selinux   | selinux process and mount labeling | <none>      |
| apparmor  | apparmor profile support           | libapparmor |

### Testing:

You can run tests for runC by using command:

```bash
# make test
```

Note that test cases are run in Docker container, so you need to install
`docker` first. And test requires mounting cgroups inside container, it's
done by docker now, so you need a docker version newer than 1.8.0-rc2.

You can also run specific test cases by:

```bash
# make test TESTFLAGS="-run=SomeTestFunction"
```

### Using:

To run a container, execute `runc start` in the bundle's root directory:
```bash
runc start
/ $ ps
PID   USER     COMMAND
1     daemon   sh
5     daemon   sh
/ $
```

### OCI Container JSON Format:

Below are sample `config.json` and `runtime.json` configuration files. It assumes that
the file-system is found in a directory called `rootfs` and there is a
user with uid and gid of `0` defined within that file-system.

`config.json`:
```json
{
	"version": "0.1.0",
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
			"name": "proc",
			"path": "/proc"
		},
		{
			"name": "dev",
			"path": "/dev"
		},
		{
			"name": "devpts",
			"path": "/dev/pts"
		},
		{
			"name": "shm",
			"path": "/dev/shm"
		},
		{
			"name": "mqueue",
			"path": "/dev/mqueue"
		},
		{
			"name": "sysfs",
			"path": "/sys"
		},
		{
			"name": "cgroup",
			"path": "/sys/fs/cgroup"
		}
	],
	"linux": {
		"capabilities": [
			"CAP_AUDIT_WRITE",
			"CAP_KILL",
			"CAP_NET_BIND_SERVICE"
		]
	}
}
```

`runtime.json`:
```json
{
	"mounts": {
		"proc": {
			"type": "proc",
			"source": "proc",
			"options": null
		},
		"dev": {
			"type": "tmpfs",
			"source": "tmpfs",
			"options": [
				"nosuid",
				"strictatime",
				"mode=755",
				"size=65536k"
			]
		},
		"devpts": {
			"type": "devpts",
			"source": "devpts",
			"options": [
				"nosuid",
				"noexec",
				"newinstance",
				"ptmxmode=0666",
				"mode=0620",
				"gid=5"
			]
		},
		"shm": {
			"type": "tmpfs",
			"source": "shm",
			"options": [
				"nosuid",
				"noexec",
				"nodev",
				"mode=1777",
				"size=65536k"
			]
		},
		"mqueue": {
			"type": "mqueue",
			"source": "mqueue",
			"options": [
				"nosuid",
				"noexec",
				"nodev"
			]
		},
		"sysfs": {
			"type": "sysfs",
			"source": "sysfs",
			"options": [
				"nosuid",
				"noexec",
				"nodev"
			]
		},
		"cgroup": {
			"type": "cgroup",
			"source": "cgroup",
			"options": [
				"nosuid",
				"noexec",
				"nodev",
				"relatime",
				"ro"
			]
		}
	},
	"hooks": {
		"prestart": null,
		"poststop": null
	},
	"linux": {
		"uidMappings": null,
		"gidMappings": null,
		"rlimits": [
			{
				"type": "RLIMIT_NOFILE",
				"hard": 1024,
				"soft": 1024
			}
		],
		"sysctl": null,
		"resources": {
			"disableOOMKiller": false,
			"memory": {
				"limit": 0,
				"reservation": 0,
				"swap": 0,
				"kernel": 0,
				"swappiness": -1
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
			"pids": {
				"limit": 0
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
		"cgroupsPath": "",
		"namespaces": [
			{
				"type": "pid",
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
		"devices": [
			{
				"path": "/dev/null",
				"type": 99,
				"major": 1,
				"minor": 3,
				"permissions": "rwm",
				"fileMode": 438,
				"uid": 0,
				"gid": 0
			},
			{
				"path": "/dev/random",
				"type": 99,
				"major": 1,
				"minor": 8,
				"permissions": "rwm",
				"fileMode": 438,
				"uid": 0,
				"gid": 0
			},
			{
				"path": "/dev/full",
				"type": 99,
				"major": 1,
				"minor": 7,
				"permissions": "rwm",
				"fileMode": 438,
				"uid": 0,
				"gid": 0
			},
			{
				"path": "/dev/tty",
				"type": 99,
				"major": 5,
				"minor": 0,
				"permissions": "rwm",
				"fileMode": 438,
				"uid": 0,
				"gid": 0
			},
			{
				"path": "/dev/zero",
				"type": 99,
				"major": 1,
				"minor": 5,
				"permissions": "rwm",
				"fileMode": 438,
				"uid": 0,
				"gid": 0
			},
			{
				"path": "/dev/urandom",
				"type": 99,
				"major": 1,
				"minor": 9,
				"permissions": "rwm",
				"fileMode": 438,
				"uid": 0,
				"gid": 0
			}
		],
		"apparmorProfile": "",
		"selinuxProcessLabel": "",
		"seccomp": {
			"defaultAction": "SCMP_ACT_ALLOW",
			"syscalls": []
		},
		"rootfsPropagation": ""
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
* Create `config.json` and `runtime.json` using the example from above.  You can also
generate a spec using `runc spec`, which will create those files for you.
* Execute `runc start` and you should be placed into a shell where you can run `ps`:
```
$ runc start
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
