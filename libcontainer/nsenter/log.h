#ifndef NSENTER_LOG_H
#define NSENTER_LOG_H

#include <stdbool.h>
#include <stdio.h>

/*
 * Log levels are the same as in logrus.
 */
#define PANIC   0
#define FATAL   1
#define ERROR   2
#define WARNING 3
#define INFO    4
#define DEBUG   5
#define TRACE   6

/*
 * Sets up logging by getting log fd and log level from the environment,
 * if available.
 */
void setup_logpipe(void);

bool log_enabled_for(int level);

void write_log(int level, const char *format, ...) __attribute__((format(printf, 2, 3)));

extern int logfd;

/* bailx logs a message to logfd (or stderr, if logfd is not available)
 * and terminates the program.
 */
#define bailx(fmt, ...)                                                     \
	do {                                                                \
		if (logfd < 0)                                              \
			fprintf(stderr, "FATAL: " fmt "\n", ##__VA_ARGS__); \
		else                                                        \
			write_log(FATAL, fmt, ##__VA_ARGS__);               \
		exit(1);                                                    \
	} while(0)

/* bail is the same as bailx, except it also adds ": %m" (errno). */
#define bail(fmt, ...) bailx(fmt ": %m", ##__VA_ARGS__)

#endif /* NSENTER_LOG_H */
