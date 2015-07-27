# Runtime and Lifecycle

## Lifecycle

### Create

Creates the container: file system, namespaces, cgroups, capabilities.

### Start (process)

Runs a process in a container. Can be invoked several times.

### Stop (process)

Not sure we need that from runc cli. Process is killed from the outside.

This event needs to be captured by runc to run onstop event handlers.
