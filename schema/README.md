# JSON schema

## Overview

This directory contains the [JSON Schema](http://json-schema.org/) for
validating the `config.json` of this container runtime specification.

The layout of the files is as follows:
* [schema.json](schema.json) - the primary entrypoint for the whole schema document
* [schema-linux.json](schema-linux.json) - this schema is for the Linux-specific sub-structure
* [schema-solaris.json](schema-solaris.json) - this schema is for the Solaris-specific sub-structure
* [defs.json](defs.json) - definitions for general types
* [defs-linux.json](defs-linux.json) - definitions for Linux-specific types
* [validate.go](validate.go) - validation utility source code


## Utility

There is also included a simple utility for facilitating validation of a
`config.json`. To build it:

```bash
export GOPATH=`mktemp -d`
go get -d ./...
go build ./validate.go
rm -rf $GOPATH
```

Or you can just use make command to create the utility:

```bash
make validate
```

Then use it like:

```bash
./validate schema.json <yourpath>/config.json
```
