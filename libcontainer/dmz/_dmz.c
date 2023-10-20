#ifdef RUNC_USE_STDLIB
#  include <unistd.h>
#else
#  include "xstat.h"
#  include "nolibc/nolibc.h"
#endif

extern char **environ;

int main(int argc, char **argv)
{
	if (argc < 1)
		return 127;
	return execve(argv[0], argv, environ);
}
