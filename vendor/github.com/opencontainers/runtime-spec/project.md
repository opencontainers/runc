# Project docs

## Release Process

* Increment version in [`specs-go/version.go`](specs-go/version.go)
* `git commit` version increment
* `git tag` the prior commit (preferably signed tag)
* `make docs` to produce PDF and HTML copies of the spec
* Make a release on [github.com/opencontainers/runtime-spec](https://github.com/opencontainers/runtime-spec/releases) for the version. Attach the produced docs.

