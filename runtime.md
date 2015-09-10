# Runtime and Lifecycle

## State

The runtime state for a container is persisted on disk so that external tools can consume and act on this information.
The runtime state is stored in a JSON encoded file.
It is recommended that this file is stored in a temporary filesystem so that it can be removed on a system reboot.
On Linux based systems the state information should be stored in `/run/opencontainer/containers`.
The directory structure for a container is `/run/opencontainer/containers/<containerID>/state.json`.
By providing a default location that container state is stored external applications can find all containers running on a system.

* **version** (string) Version of the OCI specification used when creating the container.
* **id** (string) ID is the container's ID.
* **pid** (int) Pid is the ID of the main process within the container.
* **root** (string) Root is the path to the container's bundle directory.

The ID is provided in the state because hooks will be executed with the state as the payload.
This allows the hook to perform clean and teardown logic after the runtime destroys its own state.

The root directory to the bundle is provided in the state so that consumers can find the container's configuration and rootfs where it is located on the host's filesystem.

*Example*

```json
{
    "id": "oc-container",
    "pid": 4422,
    "root": "/containers/redis"
}
```

## Lifecycle

### Create

Creates the container: file system, namespaces, cgroups, capabilities.

### Start (process)

Runs a process in a container.
Can be invoked several times.

### Stop (process)

Not sure we need that from runc cli.
Process is killed from the outside.

This event needs to be captured by runc to run onstop event handlers.

## Hooks

Hooks allow one to run code before/after various lifecycle events of the container.
Hooks MUST be called in the listed order.
The state of the container is passed to the hooks over stdin, so the hooks could get the information they need to do their work.

Hook paths are absolute and are executed from the host's filesystem.

### Pre-start

The pre-start hooks are called after the container process is spawned, but before the user supplied command is executed.
They are called after the container namespaces are created on Linux, so they provide an opportunity to customize the container.
In Linux, for e.g., the network namespace could be configured in this hook.

If a hook returns a non-zero exit code, then an error including the exit code and the stderr is returned to the caller and the container is torn down.

### Post-stop

The post-stop hooks are called after the container process is stopped.
Cleanup or debugging could be performed in such a hook.
If a hook returns a non-zero exit code, then an error is logged and the remaining hooks are executed.

*Example*

```json
    "hooks" : {
        "prestart": [
            {
                "path": "/usr/bin/fix-mounts",
                "args": ["arg1", "arg2"],
                "env":  [ "key1=value1"]
            },
            {
                "path": "/usr/bin/setup-network"
            }
        ],
        "poststop": [
            {
                "path": "/usr/sbin/cleanup.sh",
                "args": ["-f"]
            }
        ]
    }
```

`path` is required for a hook.
`args` and `env` are optional.
