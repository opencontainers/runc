% runc-features "8"

# NAME
**runc-features** - show implemented features

# SYNOPSIS
**runc features**

# DESCRIPTION
The **features** command shows the implemented features in JSON format. Features are properties of runc, such as the minimum and maximum accepted OCI versions. The implemented features may not always be available, depending on the kernel version, system libraries, CPU architecture, etc. For more information about features, you can check the features section in the runtime-spec on GitHub.

# PROPERTIES
**ociVersionMin**: Minimum OCI version that runc can accept.

**ociVersionMax**: Maximum OCI version that runc can accept.

**hooks**: List of hooks that runc supports. Consider hooks as defined stages that can run different commands.

**mountOptions**: List of available options for the runtime to mount a file system. Note that if an option is in the list, it does not necessarily mean your OS supports it.

**linux**: For runtimes that support Linux (which runc is one of), this option shows some Linux-specific properties such as namespaces, capabilities, cgroups, etc.

**annotations**: Contains arbitrary metadata about the runtime, such as the version of the runtime.

**potentiallyUnsafeConfigAnnotations**: Contains a list of values in annotations that can change the runtime's behavior. If it ends with a period, it indicates a prefix for other values.

# SEE ALSO
**runc**(8).
