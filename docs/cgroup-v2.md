# cgroup v2

runc fully supports cgroup v2 (unified mode) since v1.0.0-rc93.

To use cgroup v2, you might need to change the configuration of the host init system.
The following distributions are known to use cgroup v2 by default:
<!-- the list should be kept in sync with https://github.com/rootless-containers/rootlesscontaine.rs/blob/master/content/getting-started/common/cgroup2.md -->
- Fedora (since 31)
- Arch Linux (since April 2021)
- openSUSE Tumbleweed (since c. 2021)
- Debian GNU/Linux (since 11)
- Ubuntu (since 21.10)
- RHEL and RHEL-like distributions (since 9)

On other systemd-based distros, cgroup v2 can be enabled by adding `systemd.unified_cgroup_hierarchy=1` to the kernel cmdline.

## Am I using cgroup v2?

Yes if `/sys/fs/cgroup/cgroup.controllers` is present.

## Host Requirements
### Kernel
* Recommended version: 5.2 or later
* Minimum version: 4.15

Kernel older than 5.2 is not recommended due to lack of freezer.

Notably, kernel older than 4.15 MUST NOT be used (unless you are running containers with user namespaces), as it lacks support for controlling permissions of devices.

### Systemd
On cgroup v2 hosts, it is highly recommended to run runc with the systemd cgroup driver (`runc --systemd-cgroup`), though not mandatory.

The recommended systemd version is 244 or later. Older systemd does not support delegation of `cpuset` controller.

Make sure you also have the `dbus-user-session` (Debian/Ubuntu) or `dbus-daemon` (CentOS/Fedora) package installed, and that `dbus` is running. On Debian-flavored distros, this can be accomplished like so:

```console
$ sudo apt install -y dbus-user-session
$ systemctl --user start dbus
```

## Rootless
On cgroup v2 hosts, rootless runc can talk to systemd to get cgroup permissions to be delegated.

```console
$ runc spec --rootless
$ jq '.linux.cgroupsPath="user.slice:runc:foo"' config.json | sponge config.json
$ runc --systemd-cgroup run foo
```

The container processes are executed in a cgroup like `/user.slice/user-$(id -u).slice/user@$(id -u).service/user.slice/runc-foo.scope`.

### Configuring delegation
Typically, only `memory` and `pids` controllers are delegated to non-root users by default.

```console
$ cat /sys/fs/cgroup/user.slice/user-$(id -u).slice/user@$(id -u).service/cgroup.controllers
memory pids
```

To allow delegation of other controllers, you need to change the systemd configuration as follows:

```console
# mkdir -p /etc/systemd/system/user@.service.d
# cat > /etc/systemd/system/user@.service.d/delegate.conf << EOF
[Service]
Delegate=cpu cpuset io memory pids
EOF
# systemctl daemon-reload
```
