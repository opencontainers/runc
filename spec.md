# Open Container Initiative Runtime Specification

The [Open Container Initiative](http://www.opencontainers.org/) develops specifications for standards on Operating System process and application containers.

Protocols defined by this specification are:
* Linux containers: [runtime.md](runtime.md), [config.md](config.md), [config-linux.md](config-linux.md), and [runtime-linux.md](runtime-linux.md).
* Solaris containers: [runtime.md](runtime.md), [config.md](config.md), and [config-solaris.md](config-solaris.md).
* Windows containers: [runtime.md](runtime.md), [config.md](config.md), and [config-windows.md](config-windows.md).

# Table of Contents

- [Introduction](spec.md)
    - [Notational Conventions](#notational-conventions)
    - [Container Principles](principles.md)
- [Filesystem Bundle](bundle.md)
- [Runtime and Lifecycle](runtime.md)
    - [Linux-specific Runtime and Lifecycle](runtime-linux.md)
- [Configuration](config.md)
    - [Linux-specific Configuration](config-linux.md)
    - [Solaris-specific Configuration](config-solaris.md)
    - [Windows-specific Configuration](config-windows.md)
- [Glossary](glossary.md)

# Notational Conventions

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" are to be interpreted as described in [RFC 2119][rfc2119].

The key words "unspecified", "undefined", and "implementation-defined" are to be interpreted as described in the [rationale for the C99 standard][c99-unspecified].

An implementation is not compliant for a given CPU architecture if it fails to satisfy one or more of the MUST, REQUIRED, or SHALL requirements for the protocols it implements.
An implementation is compliant for a given CPU architecture if it satisfies all the MUST, REQUIRED, and SHALL requirements for the protocols it implements.

[c99-unspecified]: http://www.open-std.org/jtc1/sc22/wg14/www/C99RationaleV5.10.pdf#page=18
[rfc2119]: http://tools.ietf.org/html/rfc2119
