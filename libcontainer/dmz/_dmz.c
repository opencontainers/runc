#include <unistd.h>

extern char **environ;

int main(int argc, char **argv)
{
	if (argc < 1)
		return 127;
	return execve(argv[0], argv, environ);
}
