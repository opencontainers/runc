## memfd-bind ##

`runc` normally has to make a binary copy of itself (or of a smaller helper
binary called `runc-dmz`) when constructing a container process in order to
defend against certain container runtime attacks such as CVE-2019-5736.

This cloned binary only exists until the container process starts (this means
for `runc run` and `runc exec`, it only exists for a few hundred milliseconds
-- for `runc create` it exists until `runc start` is called). However, because
the clone is done using a memfd (or by creating files in directories that are
likely to be a `tmpfs`), this can lead to temporary increases in *host* memory
usage. Unless you are running on a cgroupv1 system with the cgroupv1 memory
controller enabled and the (deprecated) `memory.move_charge_at_immigrate`
enabled, there is no effect on the container's memory.

However, for certain configurations this can still be undesirable. This daemon
allows you to create a sealed memfd copy of the `runc` binary, which will cause
`runc` to skip all binary copying, resulting in no additional memory usage for
each container process (instead there is a single in-memory copy of the
binary). It should be noted that (strictly speaking) this is slightly less
secure if you are concerned about Dirty Cow-like 0-day kernel vulnerabilities,
but for most users the security benefit is identical.

The provided `memfd-bind@.service` file can be used to get systemd to manage
this daemon. You can supply the path like so:

```
% systemctl start memfd-bind@/usr/bin/runc
```

Thus, there are three ways of protecting against CVE-2019-5736, in order of how
much memory usage they can use:

* `memfd-bind` only creates a single in-memory copy of the `runc` binary (about
  10MB), regardless of how many containers are running.

* `runc-dmz` is (depending on which libc it was compiled with) between 10kB and
  1MB in size, and a copy is created once per process spawned inside a
  container by runc (both the pid1 and every `runc exec`). There are
  circumstances where using `runc-dmz` will fail in ways that runc cannot
  predict ahead of time (such as restrictive LSMs applied to containers), in
  which case users can disable it with the `RUNC_DMZ=legacy` setting.
  `runc-dmz` also requires an additional `execve` over the other options,
  though since the binary is so small the cost is probably not even noticeable.

* The classic method of making a copy of the entire `runc` binary during
  container process setup takes up about 10MB per process spawned inside the
  container by runc (both pid1 and `runc exec`).

### Caveats ###

There are several downsides with using `memfd-bind` on the `runc` binary:

* The `memfd-bind` process needs to continue to run indefinitely in order for
  the memfd reference to stay alive. If the process is forcefully killed, the
  bind-mount on top of the `runc` binary will become stale and nobody will be
  able to execute it (you can use `memfd-bind --cleanup` to clean up the stale
  mount).

* Only root can execute the cloned binary due to permission restrictions on
  accessing other process's files. More specifically, only users with ptrace
  privileges over the memfd-bind daemon can access the file (but in practice
  this is usually only root).

* When updating `runc`, the daemon needs to be stopped before the update (so
  the package manager can access the underlying file) and then restarted after
  the update.
