## systemd cgroup driver

By default, runc creates cgroups and sets cgroup limits on its own (this mode
is known as fs cgroup driver). When `--systemd-cgroup` global option is given
(as in e.g. `runc --systemd-cgroup run ...`), runc switches to systemd cgroup
driver. This document describes its features and peculiarities.

### systemd unit name and placement

When creating a container, runc requests systemd (over dbus) to create
a transient unit for the container, and place it into a specified slice.

The name of the unit and the containing slice is derived from the container
runtime spec in the following way:

1. If `Linux.CgroupsPath` is set, it is expected to be in the form
   `[slice]:[prefix]:[name]`.

   Here `slice` is a systemd slice under which the container is placed.
   If empty, it defaults to `system.slice`, except when cgroup v2 is
   used and rootless container is created, in which case it defaults
   to `user.slice`.

   Note that `slice` can contain dashes to denote a sub-slice
   (e.g. `user-1000.slice` is a correct notation, meaning a subslice
   of `user.slice`), but it must not contain slashes (e.g.
   `user.slice/user-1000.slice` is invalid).

   A `slice` of `-` represents a root slice.

   Next, `prefix` and `name` are used to compose the  unit name, which
   is `<prefix>-<name>.scope`, unless `name` has `.slice` suffix, in
   which case `prefix` is ignored and the `name` is used as is.

2. If `Linux.CgroupsPath` is not set or empty, it works the same way as if it
   would be set to `:runc:<container-id>`. See the description above to see
   what it transforms to.

As described above, a unit being created can either be a scope or a slice.
For a scope, runc specifies its parent slice via a _Slice=_ systemd property,
and also sets _Delegate=true_. For a slice, runc specifies a weak dependency on
the parent slice via a _Wants=_ property.

### Resource limits

runc always enables accounting for all controllers, regardless of any limits
being set. This means it unconditionally sets the following properties for the
systemd unit being created:

 * _CPUAccounting=true_
 * _IOAccounting=true_ (_BlockIOAccounting_ for cgroup v1)
 * _MemoryAccounting=true_
 * _TasksAccounting=true_

The resource limits of the systemd unit are set by runc by translating the
runtime spec resources to systemd unit properties.

Such translation is by no means complete, as there are some cgroup properties
that can not be set via systemd.  Therefore, runc systemd cgroup driver is
backed by fs driver (in other words, cgroup limits are first set via systemd
unit properties, and when by writing to cgroupfs files).

The set of runtime spec resources which is translated by runc to systemd unit
properties depends on kernel cgroup version being used (v1 or v2), and on the
systemd version being run. If an older systemd version (which does not support
some resources) is used, runc do not set those resources.

The following tables summarize which properties are translated.

#### cgroup v1

| runtime spec resource | systemd property name | min systemd version |
|-----------------------|-----------------------|---------------------|
| memory.limit          | MemoryLimit           |                     |
| cpu.shares            | CPUShares             |                     |
| blockIO.weight        | BlockIOWeight         |                     |
| pids.limit            | TasksMax              |                     |
| cpu.cpus              | AllowedCPUs           | v244                |
| cpu.mems              | AllowedMemoryNodes    | v244                |

#### cgroup v2

| runtime spec resource   | systemd property name | min systemd version |
|-------------------------|-----------------------|---------------------|
| memory.limit            | MemoryMax             |                     |
| memory.reservation      | MemoryLow             |                     |
| memory.swap             | MemorySwapMax         |                     |
| cpu.shares              | CPUWeight             |                     |
| pids.limit              | TasksMax              |                     |
| cpu.cpus                | AllowedCPUs           | v244                |
| cpu.mems                | AllowedMemoryNodes    | v244                |
| unified.cpu.max         | CPUQuota, CPUQuotaPeriodSec | v242          |
| unified.cpu.weight      | CPUWeight             |                     |
| unified.cpu.idle        | CPUWeight             | v252                |
| unified.cpuset.cpus     | AllowedCPUs           | v244                |
| unified.cpuset.mems     | AllowedMemoryNodes    | v244                |
| unified.memory.high     | MemoryHigh            |                     |
| unified.memory.low      | MemoryLow             |                     |
| unified.memory.min      | MemoryMin             |                     |
| unified.memory.max      | MemoryMax             |                     |
| unified.memory.swap.max | MemorySwapMax         |                     |
| unified.pids.max        | TasksMax              |                     |

For documentation on systemd unit resource properties, see
`systemd.resource-control(5)` man page.

### Auxiliary properties

Auxiliary properties of a systemd unit (as shown by `systemctl show
<unit-name>` after the container is created) can be set (or overwritten) by
adding annotations to the container runtime spec (`config.json`).

For example:

```json
        "annotations": {
                "org.systemd.property.TimeoutStopUSec": "uint64 123456789",
                "org.systemd.property.CollectMode":"'inactive-or-failed'"
        },
```

The above will set the following properties:

* `TimeoutStopSec` to 2 minutes and 3 seconds;
* `CollectMode` to "inactive-or-failed".

The values must be in the gvariant text format, as described in
[gvariant documentation](https://docs.gtk.org/glib/gvariant-text.html).

To find out which type systemd expects for a particular parameter, please
consult systemd sources.
