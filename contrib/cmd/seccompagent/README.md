# Seccomp Agent

## Warning

Please note this is an example agent, as such it is possible that specially
crafted messages can produce bad behaviour. Please use it as an example only.

## Get started

Compile runc and seccompagent:
```bash
make all
```

Prepare a container:
```bash
mkdir container-seccomp-notify
cd container-seccomp-notify
mkdir rootfs
docker export $(docker create busybox) | tar -C rootfs -xvf -
```

Edit config.json to add a seccomp profile and some commands:
```json
{
  "ociVersion": "1.0.2-dev",
  "process": {
    "args": [
      "sh",
      "-c",
      "cd /dev/shm ; touch /dev/shm/file ; set -x ; for i in `seq 1 10` ; do mkdir /dev/shm/directory$i dir$i; chmod 777 /dev/shm/file ; ls -la /dev/shm ; sleep 2 ; done"
    ]
  },
  "linux": {
    "seccomp": {
      "defaultAction": "SCMP_ACT_ALLOW",
      "listenerPath": "/run/seccomp-agent.socket",
      "listenerMetadata": "foo",
      "architectures": [
        "SCMP_ARCH_X86",
        "SCMP_ARCH_X32"
      ],
      "syscalls": [
        {
          "names": [
            "fchmod",
            "chmod",
            "mkdir"
          ],
          "action": "SCMP_ACT_NOTIFY"
        }
      ]
    }
  }
}
```

Run the seccomp agent in the background:
```bash
sudo ./contrib/cmd/seccompagent/seccompagent &
```

Start the container:
```bash
runc run mycontainerid
```
