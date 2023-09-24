# NOTE: Make sure you keep this file in sync with scripts/lib.sh.

GO ?= go
GOARCH ?= $(shell $(GO) env GOARCH)

ifneq ($(shell grep -i "ID_LIKE=.*suse" /etc/os-release),)
	# openSUSE has a custom PLATFORM
	PLATFORM ?= suse-linux
	IS_SUSE := 1
else
	PLATFORM ?= linux-gnu
endif

ifeq ($(GOARCH),$(shell GOARCH= $(GO) env GOARCH))
	# use the native CC and STRIP
	HOST :=
else ifeq ($(GOARCH),386)
	# Always use the 64-bit compiler to build the 386 binary, which works for
	# the more common cross-build method for x86 (namely, the equivalent of
	# dpkg --add-architecture).
	ifdef IS_SUSE
		# There is no x86_64-suse-linux-gcc, so use the native one.
		HOST :=
		CPU_TYPE := i586
	else
		HOST := x86_64-$(PLATFORM)-
		CPU_TYPE := i686
	endif
	CFLAGS := -m32 -march=$(CPU_TYPE) $(CFLAGS)
else ifeq ($(GOARCH),amd64)
	ifdef IS_SUSE
		# There is no x86_64-suse-linux-gcc, so use the native one.
		HOST :=
	else
		HOST := x86_64-$(PLATFORM)-
	endif
else ifeq ($(GOARCH),arm64)
	HOST := aarch64-$(PLATFORM)-
else ifeq ($(GOARCH),arm)
	# HOST already configured by release_build.sh in this case.
else ifeq ($(GOARCH),armel)
	HOST := arm-$(PLATFORM)eabi-
else ifeq ($(GOARCH),armhf)
	HOST := arm-$(PLATFORM)eabihf-
else ifeq ($(GOARCH),ppc64le)
	HOST := powerpc64le-$(PLATFORM)-
else ifeq ($(GOARCH),riscv64)
	HOST := riscv64-$(PLATFORM)-
else ifeq ($(GOARCH),s390x)
	HOST := s390x-$(PLATFORM)-
else
$(error Unsupported GOARCH $(GOARCH))
endif

ifeq ($(origin CC),$(filter $(origin CC),undefined default))
	# Override CC if it's undefined or just the default value set by Make.
	CC := $(HOST)gcc
	export CC
endif
STRIP ?= $(HOST)strip
export STRIP
