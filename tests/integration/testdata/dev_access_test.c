#include <stdio.h>
#include <unistd.h>

int main(int argc, char *argv[])
{
	const char *dev_name = "/dev/kmsg";

	if (argc > 1)
		dev_name = argv[1];

	if (access(dev_name, F_OK) < 0) {
		perror(dev_name);
		return 1;
	}

	return 0;
}
