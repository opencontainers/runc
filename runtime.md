# Open Container Runtime Specification

## Container actions

start, stop,...

## Container runtime environment

network interface, ...

## Container runtime configuration

[Docs generated from json schema](runtime-config.md)

### Configuration parameters
### Profiles
Profiles specify default parameters for running containers in specific context.

#### Untrusted profile

The code to run is not trusted at all. This profile provides a high level of isolation, you can run it in production.

#### Default profile

This profile can be used in development, it reasonably isolates the code from your infrastructure, but does assume the code you run is not actively harmful.

#### Priviledged profile

This profile is for code that you trust with root access to your system.
