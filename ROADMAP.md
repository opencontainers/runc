# OCI Specs Roadmap

This document serves to provide a long term roadmap on our quest to a 1.0 version of the OCI container specification.
Its goal is to help both maintainers and contributors find meaningful tasks to focus on and create a low noise environment.
The items in the 1.0 roadmap can be broken down into smaller milestones that are easy to accomplish.
The topics below are broad and small working groups will be needed for each to define scope and requirements or if the feature is required at all for the OCI level.
Topics listed in the roadmap do not mean that they will be implemented or added but are areas that need discussion to see if they fit in to the goals of the OCI.

Listed topics may defer to the [project wiki][runtime-wiki] for collaboration.

## 1.0

### Container Definition

Define what a software container is and its attributes in a cross platform way.

Could be solved by lifecycle/ops and create/start split discussions

*Owner:* vishh & duglin

### Version Schema

Decide on a robust versioning schema for the spec as it evolves.

Resolved but release process could evolve. Resolved for v0.2.0, expect to revisit near v1.0.0

*Owner:* vbatts

### Base Config Compatibility

Ensure that the base configuration format is viable for various platforms.

Systems:

* Linux
* Solaris
* Windows

*Owner:* robdolinms as lead coordinator

### Full Lifecycle Hooks

Ensure that we have lifecycle hooks in the correct places with full coverage over the container lifecycle.

Will probably go away with Vish's work on splitting create and start, and if we have exec.

*Owner:*


[runtime-wiki]: https://github.com/opencontainers/runtime-spec/wiki/RoadMap
