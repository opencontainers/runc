# <a name="windowsSpecificContainerConfiguration" />Windows-specific Container Configuration

This document describes the schema for the [Windows-specific section](config.md#platform-specific-configuration) of the [container configuration](config.md).
The Windows container specification uses APIs provided by the Windows Host Compute Service (HCS) to fulfill the spec.

## <a name="configWindowsResources" />Resources

You can configure a container's resource limits via the OPTIONAL `resources` field of the Windows configuration.

### <a name="configWindowsMemory" />Memory

`memory` is an OPTIONAL configuration for the container's memory usage.

The following parameters can be specified:

* **`limit`** *(uint64, OPTIONAL)* - sets limit of memory usage in bytes.

* **`reservation`** *(uint64, OPTIONAL)* - sets the guaranteed minimum amount of memory for a container in bytes.

#### Example

```json
    "windows": {
        "resources": {
            "memory": {
                "limit": 2097152,
                "reservation": 524288
            }
        }
    }
```

### <a name="configWindowsCpu" />CPU

`cpu` is an OPTIONAL configuration for the container's CPU usage.

The following parameters can be specified:

* **`count`** *(uint64, OPTIONAL)* - specifies the number of CPUs available to the container.

* **`shares`** *(uint16, OPTIONAL)* - specifies the relative weight to other containers with CPU shares.

* **`maximum`** *(uint, OPTIONAL)* - specifies the portion of processor cycles that this container can use as a percentage times 100.

#### Example

```json
    "windows": {
        "resources": {
            "cpu": {
                "maximum": 5000
            }
        }
    }
```

### <a name="configWindowsStorage" />Storage

`storage` is an OPTIONAL configuration for the container's storage usage.

The following parameters can be specified:

* **`iops`** *(uint64, OPTIONAL)* - specifies the maximum IO operations per second for the system drive of the container.

* **`bps`** *(uint64, OPTIONAL)* - specifies the maximum bytes per second for the system drive of the container.

* **`sandboxSize`** *(uint64, OPTIONAL)* - specifies the minimum size of the system drive in bytes.

#### Example

```json
    "windows": {
        "resources": {
            "storage": {
                "iops": 50
            }
        }
    }
```

### <a name="configWindowsNetwork" />Network

`network` is an OPTIONAL configuration for the container's network usage.

The following parameters can be specified:

* **`egressBandwidth`** *(uint64, OPTIONAL)* - specified the maximum egress bandwidth in bytes per second for the container.

#### Example

```json
    "windows": {
        "resources": {
            "network": {
                "egressBandwidth": 1048577
            }
        }
   }
```
