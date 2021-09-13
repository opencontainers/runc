# Seccomp Agent

## Warning

Please note this is an example agent, as such it is possible that specially
crafted messages can produce bad behaviour. Please use it as an example only.

Also, this agent is used for integration tests. Be aware that changing the
behaviour can break the integration tests.

## Get started

Compile runc and seccompagent:
```bash
make all
```

Run the seccomp agent in the background:
```bash
sudo ./contrib/cmd/seccompagent/seccompagent &
```

Prepare a container:
```bash
mkdir container-seccomp-notify
cd container-seccomp-notify
mkdir rootfs
docker export $(docker create busybox) | tar -C rootfs -xvf -
```

Then, generate a config.json by running the script gen-seccomp-example-cfg.sh
from the directory where this README.md is in the container directory you
prepared earlier (`container-seccomp-notify`).

Then start the container:
```bash
runc run mycontainerid
```

The container will output something like this:
```bash
+ cd /dev/shm
+ mkdir test-dir
+ touch test-file
+ chmod 777 test-file
chmod: changing permissions of 'test-file': No medium found
+ stat /dev/shm/test-dir-foo
  File: /dev/shm/test-dir-foo
  Size: 40        	Blocks: 0          IO Block: 4096   directory
Device: 3eh/62d	Inode: 2           Links: 2
Access: (0755/drwxr-xr-x)  Uid: (    0/    root)   Gid: (    0/    root)
Access: 2021-09-09 15:03:13.043716040 +0000
Modify: 2021-09-09 15:03:13.043716040 +0000
Change: 2021-09-09 15:03:13.043716040 +0000
 Birth: -
+ ls -l /dev/shm
total 0
drwxr-xr-x 2 root root 40 Sep  9 15:03 test-dir-foo
-rw-r--r-- 1 root root  0 Sep  9 15:03 test-file
+ echo Note the agent added a suffix for the directory name and chmod fails
Note the agent added a suffix for the directory name and chmod fails
```

This shows a simple example that runs in /dev/shm just because it is a tmpfs in
the example config.json.

The agent makes all chmod calls fail with ENOMEDIUM, as the example output shows.

For mkdir, the agent adds a "-foo" suffix: the container runs "mkdir test-dir"
but the directory created is "test-dir-foo".
