# libcontainer

[![Go Reference](https://pkg.go.dev/badge/github.com/opencontainers/runc/libcontainer.svg)](https://pkg.go.dev/github.com/opencontainers/runc/libcontainer)

Libcontainer provides a native Go implementation for creating containers
with namespaces, cgroups, capabilities, and filesystem access controls.
It allows you to manage the lifecycle of the container performing additional operations
after the container is created.


## Container
A container is a self contained execution environment that shares the kernel of the
host system and which is (optionally) isolated from other containers in the system.

## Using libcontainer

For a brief overview of using libcontainer, see [example_test.go](example_test.go).

### Container init

Because containers are spawned in a two step process you will need a binary that
will be executed as the init process for the container. In libcontainer, we use
the current binary (/proc/self/exe) to be executed as the init process, and use
arg "init", we call the first step process "bootstrap", so you always need a "init"
function as the entry of "bootstrap".

In addition to the go init function the early stage bootstrap is handled by importing
[nsenter](../nsenter/README.md).

For details on how runc implements such "init", see
[../init.go](../init.go) and [init_linux.go](init_linux.go).

## Checkpoint & Restore

libcontainer now integrates [CRIU](http://criu.org/) for checkpointing and restoring containers.
This lets you save the state of a process running inside a container to disk, and then restore
that state into a new process, on the same machine or on another machine.

`criu` version 1.5.2 or higher is required to use checkpoint and restore.
If you don't already  have `criu` installed, you can build it from source, following the
[online instructions](http://criu.org/Installation). `criu` is also installed in the docker image
generated when building libcontainer with docker.


## Copyright and license

Code and documentation copyright 2014 Docker, inc.
The code and documentation are released under the [Apache 2.0 license](../LICENSE).
The documentation is also released under Creative Commons Attribution 4.0 International License.
You may obtain a copy of the license, titled CC-BY-4.0, at http://creativecommons.org/licenses/by/4.0/.
