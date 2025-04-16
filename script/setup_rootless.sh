#!/bin/bash
set -eux -o pipefail

# Add a user for rootless tests.
sudo useradd -u2000 -m -d/home/rootless -s/bin/bash rootless

# Allow both the current user and rootless itself to use
# ssh rootless@localhost in tests/rootless.sh.
# shellcheck disable=SC2174 # Silence "-m only applies to the deepest directory".
mkdir -p -m 0700 "$HOME/.ssh"
ssh-keygen -t ecdsa -N "" -f "$HOME/.ssh/rootless.key"
sudo mkdir -p -m 0700 /home/rootless/.ssh
sudo cp "$HOME/.ssh/rootless.key" /home/rootless/.ssh/id_ecdsa
sudo cp "$HOME/.ssh/rootless.key.pub" /home/rootless/.ssh/authorized_keys
sudo chown -R rootless.rootless /home/rootless
