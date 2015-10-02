# Filesystem Bundle

## Container Format

This section defines a format for encoding a container as a *filesystem bundle* - a set of files organized in a certain way, and containing all the necessary data and metadata for any compliant runtime to perform all standard operations against it.
See also [OS X application bundles](http://en.wikipedia.org/wiki/Bundle_%28OS_X%29) for a similar use of the term *bundle*.

The definition of a bundle is only concerned with how a container, and its configuration data, are stored on a local file system so that it can be consumed by a compliant runtime.

A Standard Container bundle contains all the information needed to load and run a container.
This includes the following three artifacts which MUST all reside in the same directory on the local filesystem:

1. `config.json` : immutable, host independent configuration.
This REQUIRED file, which MUST be named `config.json`, contains settings that are host independent and application specific such as security permissions, environment variables and arguments.
See [`config.json`](config.md) for more details.

2. `runtime.json` : mutable, host dependent configuration.
This REQUIRED file, which MUST be named `runtime.json`, contains settings that are host specific such as memory limits, local device access and mount sources.
The goal is that the bundle can be moved as a unit to another runtime and run the same application if `runtime.json` is reconfigured.
See [`runtime.json`](runtime-config.md) for more details.

3. A directory representing the root filesystem of the container.
While the name of this REQUIRED directory may be arbitrary, users should consider using a conventional name, such as `rootfs`.
This directory MUST be referenced from within the `config.json` file.

While these three artifacts MUST all be present in a single directory on the local filesytem, that directory itself is not part of the bundle.
In other words, a tar archive of a *bundle* will have these artifacts at the root of the archive, not nested within a top-level directory.
