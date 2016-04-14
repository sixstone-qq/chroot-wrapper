# chroot-wrapper

This tool and library is intended to run tasks inside images downloaded from the
Internet.

The valid image formats are: tar, tar.gz
Valid transport protocols: file, http(s)

It requires Go 1.5 or higher.

## Usage

    $ go get github.com/sixstone-qq/chroot-wrapper
    $ chroot-wrapper -h

    Usage ./bin/chroot-wrapper [flags] <subcommand> [arguments]

      Available subcommands: run, ps, kill

	         [-env=[]] run URL|path cmd [args...]

             Run cmd inside an image (jailed) which is available at the given URL.
		     Only file and HTTP(S) schemes are supported.
		     Only TAR images compressed or not with GZ are supported

             ps

	         Get the status of task launched with run subcommand

	         kill [signal]

		     Send signal to the task launched with run subcommand
		     Possible signal values: SIGKILL (default), SIGTERM, SIGUSR1, SIGUSR2, SIGSTOP, SIGCONT, SIGINT

     -env string
         New environment variables available for the task
     -port int
         Supervisor listening port to query task
     -wd string
         Working directory to run the task

## Purpose

This tool is available via library with a package called task with the
struct Task in it. Use `godoc
github.com/sixstone-qq/chroot-wrapper/task` and the unit tests for
details.

The chroot to the image can be done without privileges thanks to the
usage of Linux mount namespaces which are the core essential of
containers.

## Tests

There are unit tests that are running using standard `go test` and
there are integration tests using an alpine base image from
`task/fixtures` subdirectory. In order to run them all:

    go test github.com/sixstone-qq/chroot-wrapper/task -args -test-image=task/fixtures/rootfs.tar
