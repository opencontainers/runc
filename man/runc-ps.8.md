% RUNC-PS(8) runc-ps Man Pages

# NAME
**runc** **ps** - ps displays the processes running inside a container

# SYNOPSIS
**runc** **ps** [_command options_] <_container-id_> [_format descriptors_]

# OPTIONS
**--format** _value_, **-f** _value_	select one of: _table_ (default) or _json_
   
**--list-descriptors**			print the list of supported format descriptors

[_format descriptors_] is a list of space separated AIX style format descriptors. When no format descriptors are specified, **runc** **ps** will use a set of descriptors to simulate `ps -ef` output. These are: _user_, _pid_, _ppid_, _pcpu_, _etime_, _tty_, _time_, _comm_.

The default format is table.  The following will output the processes of a container
in json format:
```
 # runc ps -f json <container-id>
```

