# Get CC values for cross-compilation.
include ../../cc_platform.mk

# We use the flags suggested in nolibc/nolibc.h, it makes the binary very small.
runc-dmz: _dmz.c
	$(CC) $(CFLAGS) -fno-asynchronous-unwind-tables -fno-ident -s -Os -nostdlib -lgcc -static -o $@ $^
	$(STRIP) -gs $@
