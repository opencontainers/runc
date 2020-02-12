## Changing systemd unit properties

In case runc uses systemd to set cgroup parameters for a container (i.e.
`--systemd-cgroup` CLI flag is set), systemd creates a scope (a.k.a.
transient unit) for the container, usually named like `runc-$ID.scope`.

The systemd properties of this unit (shown by `systemctl show runc-$ID.scope`
after the container is started) can be modified by adding annotations
to container's runtime spec (`config.json`). For example:

```json
        "annotations": {
                "org.systemd.property.TimeoutStopUSec": "uint64 123456789",
                "org.systemd.property.CollectMode":"'inactive-or-failed'"
        },
```

The above will set the following properties:

* `TimeoutStopSec` to 2 minutes and 3 seconds;
* `CollectMode` to "inactive-or-failed".

The values must be in the gvariant format (for details, see
[gvariant documentation](https://developer.gnome.org/glib/stable/gvariant-text.html)).

To find out which type systemd expects for a particular parameter, please
consult systemd sources. In particular, parameters with `USec` suffix are
in microseconds, and those require an `uint64` typed argument. Since
gvariant assumes int32 for a numeric values, the explicit type is required.

**Note** that time-typed systemd parameter names must have the `USec`
suffix, while they are documented with `Sec` suffix.
For example, the stop timeout used in the example above must be
set as `TimeoutStopUSec` but is shown and documented as `TimeoutStopSec`.
