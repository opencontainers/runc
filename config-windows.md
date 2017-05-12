# <a name="windowsSpecificContainerConfiguration" />Windows-specific Container Configuration

This document describes the schema for the [Windows-specific section](config.md#platform-specific-configuration) of the [container configuration](config.md).
The Windows container specification uses APIs provided by the Windows Host Compute Service (HCS) to fulfill the spec.

## <a name="configWindowsResources" />Resources

You can configure a container's resource limits via the OPTIONAL `resources` field of the Windows configuration.

### <a name="configWindowsMemory" />Memory

`memory` is an OPTIONAL configuration for the container's memory usage.

The following parameters can be specified:

* **`limit`** *(uint64, OPTIONAL)* - sets limit of memory usage in bytes.

#### Example

```json
    "windows": {
        "resources": {
            "memory": {
                "limit": 2097152
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

## <a name="configWindowsCredentialSpec" />Credential Spec

You can configure a container's group Managed Service Account (gMSA) via the OPTIONAL `credentialspec` field of the Windows configuration.
The `credentialspec` is a JSON object whose properties are implementation-defined.
For more information about gMSAs, see [Active Directory Service Accounts for Windows Containers][gMSAOverview].
For more information about tooling to generate a gMSA, see [Deployment Overview][gMSATooling].


[gMSAOverview]: https://aka.ms/windowscontainers/manage-serviceaccounts
[gMSATooling]: https://aka.ms/windowscontainers/credentialspec-tools

## <a name="configWindowsServicing" />Servicing

When a container terminates, the Host Compute Service indicates if a Windows update servicing operation is pending.
You can indicate that a container should be started in a mode to apply pending servicing operations via the OPTIONAL `servicing` field of the Windows configuration.

### Example

```json
    "windows": {
        "servicing": true
    }
```

## <a name="configWindowsIgnoreFlushesDuringBoot" />IgnoreFlushesDuringBoot

You can indicate that a container should be started in an a mode where disk flushes are not performed during container boot via the OPTIONAL `ignoreflushesduringboot` field of the Windows configuration.

### Example

```json
    "windows": {
        "ignoreflushesduringboot": true
    }
```