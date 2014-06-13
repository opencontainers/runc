# libcontainer: what's next?

This document is a high-level overview of where we want to take libcontainer next.
It is a curated selection of planned improvements which are either important, difficult, or both.

For a more complete view of planned and requested improvements, see [the Github issues](https://github.com/dotcloud/docker/issues).

To suggest changes to the roadmap, including additions, please write the change as if it were already in effect, and make a pull request.

## Broader kernel support

Our goal is to make libcontainer run everywhere, but currently libcontainer requires Linux version 3.8 or higher with lxc and aufs support. If you’re deploying new machines for the purpose of running libcontainer, this is a fairly easy requirement to meet. However, if you’re adding libcontainer to an existing deployment, you may not have the flexibility to update and patch the kernel.

Expanding libcontainer’s kernel support is a priority. This includes running on older kernel versions, but also on kernels with no AUFS support, or with incomplete lxc capabilities.

## Cross-architecture support

Our goal is to make libcontainer run everywhere. However currently libcontainer only runs on x86_64 systems. We plan on expanding architecture support, so that libcontainer containers can be created and used on more architectures.
