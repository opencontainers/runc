# Linux

## Linux Namespaces

```
    "namespaces": [
        "process",
        "network",
        "mount",
        "ipc",
        "uts",
        "user"
    ],
```

Namespaces for the container are specified as an array of strings under the namespaces key. The list of constants that can be used is portable across operating systems. Here is a table mapping these names to native OS equivalent.

For Linux the mapping is

* process -> pid: the process ID number space is specific to the container, meaning that processes in different PID namespaces can have the same PID

* network -> network: the container will have an isolated network stack

* mount -> mnt container can only access mounts local to itself

* ipc -> ipc processes in the container can only communicate with other processes inside same container

* uts -> uts Hostname and NIS domain name are specific to the container

* user -> user uids/gids on the host are mapped to different uids/gids in the container, so root in a container could be a non-root, unprivileged uid on the host

## Linux Control groups

## Linux Seccomp

## Linux Process Capabilities

```
   "capabilities": [
        "AUDIT_WRITE",
        "KILL",
        "NET_BIND_SERVICE"
    ],
```

capabilities is an array of Linux process capabilities. Valid values are the string after `CAP_` for capabilities defined in http://linux.die.net/man/7/capabilities

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


