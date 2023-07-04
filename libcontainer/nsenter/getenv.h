#ifndef NSENTER_GETENV_H
#define NSENTER_GETENV_H

/*
 * Returns an environment variable value as a non-negative integer, or -ENOENT
 * if the variable was not found or has an empty value.
 *
 * If the value can not be converted to an integer, or the result is out of
 * range, the function bails out.
 */
int getenv_int(const char *name);

#endif /* NSENTER_GETENV_H */
