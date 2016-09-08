# Runtime and Lifecycle

## Scope of a Container

Barring access control concerns, the entity using a runtime to create a container MUST be able to use the operations defined in this specification against that same container.
Whether other entities using the same, or other, instance of the runtime can see that container is out of scope of this specification.

## State

The state of a container MUST include, at least, the following properties:

* **`ociVersion`**: (string) is the OCI specification version used when creating the container.
* **`id`**: (string) is the container's ID.
This MUST be unique across all containers on this host.
There is no requirement that it be unique across hosts.
* **`status`**: (string) is the runtime state of the container.
The value MAY be one of:
    * `created`: the container has been created but the user-specified code has not yet been executed
    * `running`: the container has been created and the user-specified code is running
    * `stopped`: the container has been created and the user-specified code has been executed but is no longer running

  Additional values MAY be defined by the runtime, however, they MUST be used to represent new runtime states not defined above.
* **`pid`**: (int) is the ID of the container process, as seen by the host.
* **`bundlePath`**: (string) is the absolute path to the container's bundle directory.
This is provided so that consumers can find the container's configuration and root filesystem on the host.
* **`annotations`**: (map) contains the list of annotations associated with the container.
If no annotations were provided then this property MAY either be absent or an empty map.

When serialized in JSON, the format MUST adhere to the following pattern:

```json
{
    "ociVersion": "0.2.0",
    "id": "oci-container1",
    "status": "running",
    "pid": 4422,
    "bundlePath": "/containers/redis",
    "annotations": {
        "myKey": "myValue"
    }
}
```

See [Query State](#query-state) for information on retrieving the state of a container.

## Lifecycle
The lifecycle describes the timeline of events that happen from when a container is created to when it ceases to exist.

1. OCI compliant runtime's `create` command is invoked with a reference to the location of the bundle and a unique identifier.
2. The container's runtime environment MUST be created according to the configuration in [`config.json`](config.md).
   If the runtime is unable to create the environment specified in the [`config.json`](config.md), it MUST generate an error.
   While the resources requested in the [`config.json`](config.md) MUST be created, the user-specified code (from [`process`](config.md#process-configuration) MUST NOT be run at this time.
   Any updates to `config.json` after this step MUST NOT affect the container.
3. Once the container is created additional actions MAY be performed based on the features the runtime chooses to support.
   However, some actions might only be available based on the current state of the container (e.g. only available while it is started).
4. Runtime's `start` command is invoked with the unique identifier of the container.
   The runtime MUST run the user-specified code, as specified by [`process`](config.md#process-configuration).
5. The container's process is stopped.
   This MAY happen due to them erroring out, exiting, crashing or the runtime's `kill` operation being invoked.
6. Runtime's `delete` command is invoked with the unique identifier of the container.
   The container MUST be destroyed by undoing the steps performed during create phase (step 2).

## Errors

In cases where the specified operation generates an error, this specification does not mandate how, or even if, that error is returned or exposed to the user of an implementation.
Unless otherwise stated, generating an error MUST leave the state of the environment as if the operation were never attempted - modulo any possible trivial ancillary changes such as logging.

## Operations

OCI compliant runtimes MUST support the following operations, unless the operation is not supported by the base operating system.

Note: these operations are not specifying any command-line APIs, and the paramenters are inputs for general operations.

### Query State

`state <container-id>`

This operation MUST generate an error if it is not provided the ID of a container.
Attempting to query a container that does not exist MUST generate an error.
This operation MUST return the state of a container as specified in the [State](#state) section.

### Create

`create <container-id> <path-to-bundle>`

This operation MUST generate an error if it is not provided a path to the bundle and the container ID to associate with the container.
If the ID provided is not unique across all containers within the scope of the runtime, or is not valid in any other way, the implementation MUST generate an error and a new container MUST NOT be created.
Using the data in [`config.json`](config.md), this operation MUST create a new container.
This means that all of the resources associated with the container MUST be created, however, the user-specified code MUST NOT be run at this time.
If the runtime cannot create the container as specified in `config.md`, it MUST generate an error and a new container MUST NOT be created.

Upon successful completion of this operation the `status` property of this container MUST be `created`.

The runtime MAY validate `config.json` against this spec, either generically or with respect to the local system capabilities, before creating the container ([step 2](#lifecycle)).
Runtime callers who are interested in pre-create validation can run [bundle-validation tools](implementations.md#testing--tools) before invoking the create operation.

Any changes made to the [`config.json`](config.md) file after this operation will not have an effect on the container.

### Start
`start <container-id>`

This operation MUST generate an error if it is not provided the container ID.
Attempting to start a container that does not exist MUST generate an error.
Attempting to start an already started container MUST have no effect on the container and MUST generate an error.
This operation MUST run the user-specified code as specified by [`process`](config.md#process-configuration).

Upon successful completion of this operation the `status` property of this container MUST be `running`.

### Kill
`kill <container-id> <signal>`

This operation MUST generate an error if it is not provided the container ID.
Attempting to send a signal to a container that is not running MUST have no effect on the container and MUST generate an error.
This operation MUST send the specified signal to the process in the container.

When the process in the container is stopped, irrespective of it being as a result of a `kill` operation or any other reason, the `status` property of this container MUST be `stopped`.

### Delete
`delete <container-id>`

This operation MUST generate an error if it is not provided the container ID.
Attempting to delete a container that does not exist MUST generate an error.
Attempting to delete a container whose process is still running MUST generate an error.
Deleting a container MUST delete the resources that were created during the `create` step.
Note that resources associated with the container, but not created by this container, MUST NOT be deleted.
Once a container is deleted its ID MAY be used by a subsequent container.


## Hooks
Many of the operations specified in this specification have "hooks" that allow for additional actions to be taken before or after each operation.
See [runtime configuration for hooks](./config.md#hooks) for more information.
