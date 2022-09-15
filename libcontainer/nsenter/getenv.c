#define _GNU_SOURCE
#include <errno.h>
#include <stdlib.h>
#include "getenv.h"
#include "log.h"

int getenv_int(const char *name)
{
	char *val, *endptr;
	int ret;

	val = getenv(name);
	/* Treat empty value as unset variable. */
	if (val == NULL || *val == '\0')
		return -ENOENT;

	ret = strtol(val, &endptr, 10);
	if (val == endptr || *endptr != '\0')
		bail("unable to parse %s=%s", name, val);
	/*
	 * Sanity check: this must be a non-negative number.
	 */
	if (ret < 0)
		bail("bad value for %s=%s (%d)", name, val, ret);

	return ret;
}
