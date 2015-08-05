#!/usr/bin/env python
import sys
import json
import os

def touch(filepath):
	if os.path.exists(filepath):
		os.utime(filepath, None)
	else:
		open(filepath, 'a').close()

if __name__ == "__main__":
	rootfs = json.load(sys.stdin)["config"]["rootfs"]
	touch(os.path.join(rootfs, "tmp.txt"))

