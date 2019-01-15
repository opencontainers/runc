# Checkpoint and Restore #

For a basic description about checkpointing and restoring containers with
`runc` please see [runc-checkpoint(8)](../man/runc-checkpoint.8.md) and
[runc-restore(8)](../man/runc-restore.8.md).

## Checkpoint/Restore Annotations ##

In addition to specifying options on the command-line like it is described
in the man-pages (see above), it is also possible to influence CRIU's
behaviour using CRIU configuration files. For details about CRIU's
configuration file support please see [CRIU's wiki](https://criu.org/Configuration_files).

In addition to CRIU's default configuration files `runc` tells CRIU to
also evaluate the file `/etc/criu/runc.conf`. Using the annotation
`org.criu.config` it is, however, possible to change this additional
CRIU configuration file.

If the annotation `org.criu.config` is set to an empty string `runc`
will not pass any additional configuration file to CRIU. With an empty
string it is therefore possible to disable the additional CRIU configuration
file. This can be used to make sure that no additional configuration file
changes CRIU's behaviour accidentally.

If the annotation `org.criu.config` is set to a non-empty string `runc` will
pass that string to CRIU to be evaluated as an additional configuration file.
If CRIU cannot open this additional configuration file, it will ignore this
file and continue.

### Annotation Example to disable additional CRIU configuration file ###

```
{
	"ociVersion": "1.0.0",
	"annotations": {
		"org.criu.config": ""
	},
	"process": {
```

### Annotation Example to set a specific CRIU configuration file ###

```
{
	"ociVersion": "1.0.0",
	"annotations": {
		"org.criu.config": "/etc/special-runc-criu-options"
	},
	"process": {
```
