# Get CC values for cross-compilation.
include ../../cc_platform.mk

runc-dmz: _dmz.c
	$(CC) $(CFLAGS) -static -o $@ $^
	$(STRIP) -gs $@
