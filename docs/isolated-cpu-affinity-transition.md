## Isolated CPU affinity transition

The introduction of the kernel commit 46a87b3851f0d6eb05e6d83d5c5a30df0eca8f76
in 5.7 has affected a deterministic scheduling behavior by distributing tasks
across CPU cores within a cgroups cpuset. It means that `runc exec` might be
impacted under some circumstances, by example when a container has been
created within a cgroup cpuset entirely composed of isolated CPU cores
usually sets either with `nohz_full` and/or `isolcpus` kernel boot parameters.

Some containerized real-time applications are relying on this deterministic
behavior and uses the first CPU core to run a slow thread while other CPU
cores are fully used by the real-time threads with SCHED_FIFO policy.
Such applications can prevent runc process from joining a container when the
runc process is randomly scheduled on a CPU core owned by a real-time thread.

Runc introduces a way to restore this behavior by adding the following
annotation to the container runtime spec (`config.json`):

`org.opencontainers.runc.exec.isolated-cpu-affinity-transition`

This annotation can take one of those values:

* `temporary` to temporarily set the runc process CPU affinity to the first
isolated CPU core of the container cgroup cpuset.
* `definitive`: to definitively set the runc process CPU affinity to the first
isolated CPU core of the container cgroup cpuset.

For example:

```json
  "annotations": {
    "org.opencontainers.runc.exec.isolated-cpu-affinity-transition": "temporary"
  }
```

__WARNING:__ `definitive` requires a kernel >= 6.2, also works with RHEL 9 and
above.

### How it works?

When enabled and during `runc exec`, runc is looking for the `nohz_full` kernel
boot parameter value and considers the CPUs in the list as isolated, it doesn't
look for `isolcpus` boot parameter, it just assumes that `isolcpus` value is
identical to `nohz_full` when specified. If `nohz_full` parameter is not found,
runc also attempts to read the list from `/sys/devices/system/cpu/nohz_full`.

Once it gets the isolated CPU list, it returns an eligible CPU core within the
container cgroup cpuset based on those heuristics:

* when there is not cpuset cores: no eligible CPU
* when there is not isolated cores: no eligible CPU
* when cpuset cores are not in isolated core list: no eligible CPU
* when cpuset cores are all isolated cores: return the first CPU of the cpuset
* when cpuset cores are mixed between housekeeping/isolated cores: return the
  first housekeeping CPU not in isolated CPUs.

The returned CPU core is then used to set the `runc init` CPU affinity before
the container cgroup cpuset transition.

#### Transition example

`nohz_full` has the isolated cores `4-7`. A container has been created with
the cgroup cpuset `4-7` to only run on the isolated CPU cores 4 to 7.
`runc exec` is called by a process with CPU affinity set to `0-3`

* with `temporary` transition:

  runc exec (affinity 0-3) -> runc init (affinity 4) -> container process (affinity 4-7)

* with `definitive` transition:

  runc exec (affinity 0-3) -> runc init (affinity 4) -> container process (affinity 4)

The difference between `temporary` and `definitive` is the container process
affinity, `definitive` will constraint the container process to run on the
first isolated CPU core of the cgroup cpuset, while `temporary` restore the
CPU affinity to match the container cgroup cpuset.

`definitive` transition might be helpful when `nohz_full` is used without
`isolcpus` to avoid runc and container process to be a noisy neighbour for
real-time applications.

### How to use it with Kubernetes?

Kubernetes doesn't manage container directly, instead it uses the Container Runtime
Interface (CRI) to communicate with a software implementing this interface and responsible
to manage the lifecycle of containers. There are popular CRI implementations like Containerd
and CRI-O. Those implementations allows to pass pod annotations to the container runtime
via the container runtime spec. Currently runc is the runtime used by default for both.

#### Containerd configuration

Containerd CRI uses runc by default but requires an extra step to pass the annotation to runc.
You have to whitelist `org.opencontainers.runc.exec.isolated-cpu-affinity-transition` as a pod
annotation allowed to be passed to the container runtime in `/etc/containerd/config.toml`:

```toml
[plugins."io.containerd.grpc.v1.cri".containerd]
  default_runtime_name = "runc"
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
      base_runtime_spec = "/etc/containerd/cri-base.json"
      pod_annotations = ["org.opencontainers.runc.exec.isolated-cpu-affinity-transition"]
```

#### CRI-O configuration

CRI-O doesn't require any extra step, however some annotations could be excluded by
configuration.

#### Pod deployment example

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: demo-pod
  annotations:
    org.opencontainers.runc.exec.isolated-cpu-affinity-transition: "temporary"
spec:
  containers:
  - name: demo
    image: registry.com/demo:latest
```
