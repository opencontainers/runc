# Get CC values for cross-compilation.
include ../../cc_platform.mk

runc-dmz: _dmz.c _dmz_arch.h
	$(CC) $(CFLAGS) -s -Os -nostartfiles -nostdlib -static -o $@ $<
	$(STRIP) -gs $@
