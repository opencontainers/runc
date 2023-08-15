#include <unistd.h>

extern char **environ;

int main(int argv, char **args)
{
    if (argv > 0) {
        return execve(args[0], args, environ);
    }
    return 0;
}
