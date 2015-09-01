# Bundle Container Format

This section defines a format for encoding a container as a *bundle* - a directory organized in a certain way, and containing all the necessary data and metadata for any compliant runtime to perform all standard operations against it.
See also [OS X application bundles](http://en.wikipedia.org/wiki/Bundle_%28OS_X%29) for a similar use of the term *bundle*.

The format does not define distribution.
In other words, it only specifies how a container must be stored on a local filesystem, for consumption by a runtime.
It does not specify how to transfer a container between computers, how to discover containers, or assign names or versions to them.
Any distribution method capable of preserving the original layout of a container, as specified here, is considered compliant.

A standard container bundle is made of the following 3 parts:

- A top-level directory holding everything else
- One or more content directories
- A configuration file

# Directory layout

A Standard Container bundle is a directory containing all the content needed to load and run a container.
This includes two configuration files `config.json` and `runtime.json`, and a rootfs directory.
The `config.json` file contains settings that are host independent and application specific such as security permissions, environment variables and arguments.
The `runtime.json` file contains settings that are host specific such as memory limits, local device access and mount points.
The goal is that the bundle can be moved as a unit to another machine and run the same application if `runtime.json` is removed or reconfigured.

The syntax and semantics for `config.json` are described in [this specification](config.md).

A single `rootfs` directory MUST be in the same directory as the `config.json`.
The names of the directories may be arbitrary, but users should consider using conventional names as in the example below.

```
config.json
runtime.json
rootfs/
```
