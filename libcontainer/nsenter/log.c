#define _GNU_SOURCE
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "log.h"
#include "getenv.h"

static const char *level_str[] = { "panic", "fatal", "error", "warning", "info", "debug", "trace" };

int logfd = -1;
static int loglevel = DEBUG;

extern char *escape_json_string(char *str);
void setup_logpipe(void)
{
	int i;

	i = getenv_int("_LIBCONTAINER_LOGPIPE");
	if (i < 0) {
		/* We are not runc init, or log pipe was not provided. */
		return;
	}
	logfd = i;

	i = getenv_int("_LIBCONTAINER_LOGLEVEL");
	if (i < 0)
		return;
	loglevel = i;
}

/* Defined in nsexec.c */
extern int current_stage;

void write_log(int level, const char *format, ...)
{
	char *message = NULL, *stage = NULL, *json = NULL;
	va_list args;
	int ret;

	if (logfd < 0 || level > loglevel)
		goto out;

	va_start(args, format);
	ret = vasprintf(&message, format, args);
	va_end(args);
	if (ret < 0) {
		message = NULL;
		goto out;
	}

	message = escape_json_string(message);

	if (current_stage < 0) {
		stage = strdup("nsexec");
		if (stage == NULL)
			goto out;
	} else {
		ret = asprintf(&stage, "nsexec-%d", current_stage);
		if (ret < 0) {
			stage = NULL;
			goto out;
		}
	}
	ret = asprintf(&json, "{\"level\":\"%s\", \"msg\": \"%s[%d]: %s\"}\n",
		       level_str[level], stage, getpid(), message);
	if (ret < 0) {
		json = NULL;
		goto out;
	}

	/* This logging is on a best-effort basis. In case of a short or failed
	 * write there is nothing we can do, so just ignore write() errors.
	 */
	ssize_t __attribute__((unused)) __res = write(logfd, json, ret);

out:
	free(message);
	free(stage);
	free(json);
}
