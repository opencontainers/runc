#include <stdlib.h>

static char hex(char i)
{
	if (i >= 0 && i < 10) {
		return '0' + i;
	}
	if (i >= 10 && i < 16) {
		return 'a' + i - 10;
	}
	return '?';
}

/*
 * Escape the string to be usable as JSON string.
 */
char *escape_json_string(char *str)
{
	int i, j = 0;
	char *out;

	// Avoid malloc by checking first if escaping is required.
	// While at it, count how much additional space we need.
	// XXX: the counting code need to be in sync with the rest!
	for (i = 0; str[i] != '\0'; i++) {
		switch (str[i]) {
		case '\\':
		case '"':
		case '\b':
		case '\n':
		case '\r':
		case '\t':
		case '\f':
			j += 2;
			break;
		default:
			if (str[i] < ' ') {
				// \u00xx
				j += 6;
			}
		}
	}
	if (j == 0) {
		// nothing to escape
		return str;
	}

	out = malloc(i + j);
	if (!out) {
		exit(1);
	}
	for (i = j = 0; str[i] != '\0'; i++, j++) {
		switch (str[i]) {
		case '"':
		case '\\':
			out[j++] = '\\';
			out[j] = str[i];
			continue;
		}
		if (str[i] >= ' ') {
			out[j] = str[i];
			continue;
		}
		out[j++] = '\\';
		switch (str[i]) {
		case '\b':
			out[j] = 'b';
			break;
		case '\n':
			out[j] = 'n';
			break;
		case '\r':
			out[j] = 'r';
			break;
		case '\t':
			out[j] = 't';
			break;
		case '\f':
			out[j] = 'f';
			break;
		default:
			out[j++] = 'u';
			out[j++] = '0';
			out[j++] = '0';
			out[j++] = hex(str[i] >> 4);
			out[j] = hex(str[i] & 0x0f);
		}
	}
	out[j] = '\0';

	free(str);
	return out;
}
