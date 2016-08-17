# <a name="runtimeAndLifecycle" />Runtime and Lifecycle

## <a name="runtimeScopeContainer" />Scope of a Container

Barring access control concerns, the entity using a runtime to create a container MUST be able to use the operations defined in this specification against that same container.
Whether other entities using the same, or other, instance of the runtime can see that container is out of scope of this specification.

## <a name="runtimeState" />State

The state of a container includes the following properties:

* **`ociVersion`** (string, REQUIRED) is the OCI specification version used when creating the container.
* **`id`** (string, REQUIRED) is the container's ID.
This MUST be unique across all containers on this host.
There is no requirement that it be unique across hosts.
* **`status`** (string, REQUIRED) is the runtime state of the container.
The value MAY be one of:

    * `creating`: the container is being created (step 2 in the [lifecycle](#lifecycle))
    * `created`: the runtime has finished the [create operation](#create) (after step 2 in the [lifecycle](#lifecycle)), and the container process has neither exited nor executed the user-specified program
    * `running`: the container process has executed the user-specified program but has not exited (after step 4 in the [lifecycle](#lifecycle))
    * `stopped`: the container process has exited (step 5 in the [lifecycle](#lifecycle))

    Additional values MAY be defined by the runtime, however, they MUST be used to represent new runtime states not defined above.
* **`pid`** (int, REQUIRED when `status` is `created` or `running`) is the ID of the container process, as seen by the host.
* **`bundle`** (string, REQUIRED) is the absolute path to the container's bundle directory.
This is provided so that consumers can find the container's configuration and root filesystem on the host.
* **`annotations`** (map, OPTIONAL) contains the list of annotations associated with the container.
If no annotations were provided then this property MAY either be absent or an empty map.

The state MAY include additional properties.

When serialized in JSON, the format MUST adhere to the following pattern:

```json
{
    "ociVersion": "0.2.0",
    "id": "oci-container1",
    "status": "running",
    "pid": 4422,
    "bundle": "/containers/redis",
    "annotations": {
        "myKey": "myValue"
    }
}
```

See [Query State](#query-state) for information on retrieving the state of a container.

## <a name="runtimeLifecycle" />Lifecycle
The lifecycle describes the timeline of events that happen from when a container is created to when it ceases to exist.

1. OCI compliant runtime's [`create`](runtime.md#create) command is invoked with a reference to the location of the bundle and a unique identifier.
2. The container's runtime environment MUST be created according to the configuration in [`config.json`](config.md).
   If the runtime is unable to create the environment specified in the [`config.json`](config.md), it MUST [generate an error](#errors).
   While the resources requested in the [`config.json`](config.md) MUST be created, the user-specified program (from [`process`](config.md#process)) MUST NOT be run at this time.
   Any updates to [`config.json`](config.md) after this step MUST NOT affect the container.
3. Once the container is created additional actions MAY be performed based on the features the runtime chooses to support.
   However, some actions might only be available based on the current state of the container (e.g. only available while it is started).
4. Runtime's [`start`](runtime.md#start) command is invoked with the unique identifier of the container.
5. The [prestart hooks](config.md#prestart) MUST be invoked by the runtime.
   If any prestart hook fails, the runtime MUST [generate an error](#errors), stop the container, and continue the lifecycle at step 10.
6. The runtime MUST run the user-specified program, as specified by [`process`](config.md#process).
7. The [poststart hooks](config.md#poststart) MUST be invoked by the runtime.
   If any poststart hook fails, the runtime MUST [log a warning](#warnings), but the remaining hooks and lifecycle continue as if the hook had succeeded.
8. The container process exits.
   This MAY happen due to erroring out, exiting, crashing or the runtime's [`kill`](runtime.md#kill) operation being invoked.
9. Runtime's [`delete`](runtime.md#delete) command is invoked with the unique identifier of the container.
10. The container MUST be destroyed by undoing the steps performed during create phase (step 2).
11. The [poststop hooks](config.md#poststop) MUST be invoked by the runtime.
    If any poststop hook fails, the runtime MUST [log a warning](#warnings), but the remaining hooks and lifecycle continue as if the hook had succeeded.

## <a name="runtimeErrors" />Errors

In cases where the specified operation generates an error, this specification does not mandate how, or even if, that error is returned or exposed to the user of an implementation.
Unless otherwise stated, generating an error MUST leave the state of the environment as if the operation were never attempted - modulo any possible trivial ancillary changes such as logging.

## <a name="runtimeWarnings" />Warnings

In cases where the specified operation logs a warning, this specification does not mandate how, or even if, that warning is returned or exposed to the user of an implementation.
Unless otherwise stated, logging a warning does not change the flow of the operation; it MUST continue as if the warning had not been logged.

## <a name="runtimeOperations" />Operations

OCI compliant runtimes MUST support the following operations, unless the operation is not supported by the base operating system.

Note: these operations are not specifying any command-line APIs, and the parameters are inputs for general operations.

### <a name="runtimeQueryState" />Query State

`state <container-id>`

This operation MUST [generate an error](#errors) if it is not provided the ID of a container.
Attempting to query a container that does not exist MUST [generate an error](#errors).
This operation MUST return the state of a container as specified in the [State](#state) section.

### <a name="runtimeCreate" />Create

`create <container-id> <path-to-bundle>`

This operation MUST [generate an error](#errors) if it is not provided a path to the bundle and the container ID to associate with the container.
If the ID provided is not unique across all containers within the scope of the runtime, or is not valid in any other way, the implementation MUST [generate an error](#errors) and a new container MUST NOT be created.
Using the data in [`config.json`](config.md), this operation MUST create a new container.
This means that all of the resources associated with the container MUST be created, however, the user-specified program MUST NOT be run at this time.
If the runtime cannot create the container as specified in [`config.json`](config.md), it MUST [generate an error](#errors) and a new container MUST NOT be created.

Upon successful completion of this operation the `status` property of this container MUST be `created`.

The runtime MAY validate `config.json` against this spec, either generically or with respect to the local system capabilities, before creating the container ([step 2](#lifecycle)).
Runtime callers who are interested in pre-create validation can run [bundle-validation tools](implementations.md#testing--tools) before invoking the create operation.

Any changes made to the [`config.json`](config.md) file after this operation will not have an effect on the container.

### <a name="runtimeStart" />Start
`start <container-id>`

This operation MUST [generate an error](#errors) if it is not provided the container ID.
Attempting to start a container that does not exist MUST [generate an error](#errors).
Attempting to start an already started container MUST have no effect on the container and MUST [generate an error](#errors).
This operation MUST run the user-specified program as specified by [`process`](config.md#process).

Upon successful completion of this operation the `status` property of this container MUST be `running`.

### <a name="runtimeKill" />Kill
`kill <container-id> <signal>`

This operation MUST [generate an error](#errors) if it is not provided the container ID.
Attempting to send a signal to a container that is not running MUST have no effect on the container and MUST [generate an error](#errors).
This operation MUST send the specified signal to the process in the container.

When the process in the container is stopped, irrespective of it being as a result of a `kill` operation or any other reason, the `status` property of this container MUST be `stopped`.

### <a name="runtimeDelete" />Delete
`delete <container-id>`

This operation MUST [generate an error](#errors) if it is not provided the container ID.
Attempting to delete a container that does not exist MUST [generate an error](#errors).
Attempting to delete a container whose process is still running MUST [generate an error](#errors).
Deleting a container MUST delete the resources that were created during the `create` step.
Note that resources associated with the container, but not created by this container, MUST NOT be deleted.
Once a container is deleted its ID MAY be used by a subsequent container.


## <a name="runtimeHooks" />Hooks
Many of the operations specified in this specification have "hooks" that allow for additional actions to be taken before or after each operation.
See [runtime configuration for hooks](./config.md#hooks) for more information.
