#!/bin/bash
set -eux -o pipefail

cat >/etc/systemd/system/machine-runc.slice <<EOF
[Unit]
Description=runc containers
Before=slices.target

[Slice]
Delegate=yes
EOF

systemctl daemon-reload
systemctl start machine-runc.slice
systemctl status machine-runc.slice
