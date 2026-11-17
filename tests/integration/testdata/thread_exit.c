/*
 * Start a thread that runs forever, then end the main thread. This leaves
 * a zombie thread group leader with a live thread, i.e. a process which
 * looks dead in /proc/PID/stat but is not.
 */
#include <pthread.h>
#include <unistd.h>

static void *loop(void *arg)
{
	(void)arg;
	for (;;)
		pause();
	return NULL;
}

int main(void)
{
	pthread_t t;

	if (pthread_create(&t, NULL, loop, NULL) != 0)
		return 1;
	pthread_exit(NULL);
}
