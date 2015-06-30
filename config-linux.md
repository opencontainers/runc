# Linux

## Linux Namespaces
```json
    "namespaces": [
        {
            "type": "pid",
            "path": "/proc/1234/ns/pid"
        },
        {
            "type": "net",
            "path": "/var/run/netns/neta"
        },
        {
            "type": "mnt",
        },
        {
            "type": "ipc",
        },
        {
            "type": "uts",
        },
        {
            "type": "user",
        },
    ]
```

A namespace wraps a global system resource in an abstraction that makes it appear to the processes within the namespace that they have their own isolated instance of the global resource.  Changes to the global resource are visible to other processes that are members of the namespace, but are invisible to other processes. For more information, see http://man7.org/linux/man-pages/man7/namespaces.7.html

Namespaces are specified in the spec as an array of entries. Each entry has a type field with possible values described below and an optional path element. If a path is specified, that particular fd is used to join that type of namespace.

* pid: the process ID number space is specific to the container, meaning that processes in different PID namespaces can have the same PID

* network: the container will have an isolated network stack

* mnt: container can only access mounts local to itself

* ipc: processes in the container can only communicate with other processes inside same container

* uts: Hostname and NIS domain name are specific to the container

* user: uids/gids on the host are mapped to different uids/gids in the container, so root in a container could be a non-root, unprivileged uid on the host

### Access to devices
```json
   "devices": [
        "null",
        "random",
        "full",
        "tty",
        "zero",
        "urandom"
    ]
```

Devices is an array specifying the list of devices from the host to make available in the container.

The array contains names: for each name, the device /dev/<name> will be made available inside the container.

## Linux Control groups

## Linux Seccomp

## Linux Process Capabilities

```json
   "capabilities": [
        "AUDIT_WRITE",
        "KILL",
        "NET_BIND_SERVICE"
    ]
```

capabilities is an array of Linux process capabilities. Valid values are the string after `CAP_` for capabilities defined in http://man7.org/linux/man-pages/man7/capabilities.7.html

## Linux Sysctl

```
   "sysctl": {
        "net.ipv4.ip_forward": "1",
        "net.core.somaxconn": "256"
   }
```

sysctl allows kernel parameters to be modified at runtime. For more information, see http://man7.org/linux/man-pages/man8/sysctl.8.html

## SELinux

## Apparmor

