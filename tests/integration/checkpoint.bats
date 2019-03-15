#!/usr/bin/env bats

load helpers

function setup() {
  if [[ -n "${RUNC_USE_SYSTEMD}" ]] ; then
    skip "CRIU test suite is skipped on systemd cgroup driver for now."
  fi

  teardown_busybox
  setup_busybox
}

function teardown() {
  teardown_busybox
}

@test "checkpoint and restore" {
  # XXX: currently criu require root containers.
  requires criu root

  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  testcontainer test_busybox running

  for i in `seq 2`; do
    # checkpoint the running container
    runc --criu "$CRIU" checkpoint --work-path ./work-dir test_busybox
    ret=$?
    # if you are having problems getting criu to work uncomment the following dump:
    #cat /run/opencontainer/containers/test_busybox/criu.work/dump.log
    cat ./work-dir/dump.log | grep -B 5 Error || true
    [ "$ret" -eq 0 ]

    # after checkpoint busybox is no longer running
    runc state test_busybox
    [ "$status" -ne 0 ]

    # restore from checkpoint
    runc --criu "$CRIU" restore -d --work-path ./work-dir --console-socket $CONSOLE_SOCKET test_busybox
    ret=$?
    cat ./work-dir/restore.log | grep -B 5 Error || true
    [ "$ret" -eq 0 ]

    # busybox should be back up and running
    testcontainer test_busybox running
  done
}

@test "checkpoint --pre-dump and restore" {
  # XXX: currently criu require root containers.
  requires criu root

  # The changes to 'terminal' are needed for running in detached mode
  sed -i 's;"terminal": true;"terminal": false;' config.json
  sed -i 's/"sh"/"sh","-c","for i in `seq 10`; do read xxx || continue; echo ponG $xxx; done"/' config.json

  # The following code creates pipes for stdin and stdout.
  # CRIU can't handle fifo-s, so we need all these tricks.
  fifo=`mktemp -u /tmp/runc-fifo-XXXXXX`
  mkfifo $fifo

  # stdout
  cat $fifo | cat $fifo &
  pid=$!
  exec 50</proc/$pid/fd/0
  exec 51>/proc/$pid/fd/0

  # stdin
  cat $fifo | cat $fifo &
  pid=$!
  exec 60</proc/$pid/fd/0
  exec 61>/proc/$pid/fd/0

  echo -n > $fifo
  unlink $fifo

  # run busybox
  __runc run -d test_busybox <&60 >&51 2>&51
  [ $? -eq 0 ]

  testcontainer test_busybox running

  #test checkpoint pre-dump
  mkdir parent-dir
  runc --criu "$CRIU" checkpoint --pre-dump --image-path ./parent-dir test_busybox
  [ "$status" -eq 0 ]

  # busybox should still be running
  runc state test_busybox
  [ "$status" -eq 0 ]
  [[ "${output}" == *"running"* ]]

  # checkpoint the running container
  mkdir image-dir
  mkdir work-dir
  runc --criu "$CRIU" checkpoint --parent-path ./parent-dir --work-path ./work-dir --image-path ./image-dir test_busybox
  cat ./work-dir/dump.log | grep -B 5 Error || true
  [ "$status" -eq 0 ]

  # after checkpoint busybox is no longer running
  runc state test_busybox
  [ "$status" -ne 0 ]

  # restore from checkpoint
  __runc --criu "$CRIU" restore -d --work-path ./work-dir --image-path ./image-dir test_busybox <&60 >&51 2>&51
  ret=$?
  cat ./work-dir/restore.log | grep -B 5 Error || true
  [ $ret -eq 0 ]

  # busybox should be back up and running
  testcontainer test_busybox running

  runc exec --cwd /bin test_busybox echo ok
  [ "$status" -eq 0 ]
  [[ ${output} == "ok" ]]

  echo Ping >&61
  exec 61>&-
  exec 51>&-
  run cat <&50
  [ "$status" -eq 0 ]
  [[ "${output}" == *"ponG Ping"* ]]
}

@test "checkpoint --lazy-pages and restore" {
  # XXX: currently criu require root containers.
  requires criu root

  # check if lazy-pages is supported
  run ${CRIU} check --feature uffd-noncoop
  if [ "$status" -eq 1 ]; then
    # this criu does not support lazy migration; skip the test
    skip "this criu does not support lazy migration"
  fi

  # The changes to 'terminal' are needed for running in detached mode
  sed -i 's;"terminal": true;"terminal": false;' config.json
  # This should not be necessary: https://github.com/checkpoint-restore/criu/issues/575
  sed -i 's;"readonly": true;"readonly": false;' config.json
  sed -i 's/"sh"/"sh","-c","for i in `seq 10`; do read xxx || continue; echo ponG $xxx; done"/' config.json

  # The following code creates pipes for stdin and stdout.
  # CRIU can't handle fifo-s, so we need all these tricks.
  fifo=`mktemp -u /tmp/runc-fifo-XXXXXX`
  mkfifo $fifo

  # For lazy migration we need to know when CRIU is ready to serve
  # the memory pages via TCP.
  lazy_pipe=`mktemp -u /tmp/lazy-pipe-XXXXXX`
  mkfifo $lazy_pipe

  # TCP port for lazy migration
  port=27277

  # stdout
  cat $fifo | cat $fifo &
  pid=$!
  exec 50</proc/$pid/fd/0
  exec 51>/proc/$pid/fd/0

  # stdin
  cat $fifo | cat $fifo &
  pid=$!
  exec 60</proc/$pid/fd/0
  exec 61>/proc/$pid/fd/0

  echo -n > $fifo
  unlink $fifo

  # run busybox
  __runc run -d test_busybox <&60 >&51 2>&51
  [ $? -eq 0 ]

  testcontainer test_busybox running

  # checkpoint the running container
  mkdir image-dir
  mkdir work-dir
  # Double fork taken from helpers.bats
  # We need to start 'runc checkpoint --lazy-pages' in the background,
  # so we double fork in the shell.
  (runc --criu "$CRIU" checkpoint --lazy-pages --page-server 0.0.0.0:${port} --status-fd ${lazy_pipe} --work-path ./work-dir --image-path ./image-dir test_busybox & ) &
  # Sleeping here. This is ugly, but not sure how else to handle it.
  # The return code of the in the background running runc is needed, if
  # there is some basic error. If the lazy migration is ready can
  # be handled by $lazy_pipe. Which probably will always be ready
  # after sleeping two seconds.
  sleep 2
  # Check if inventory.img was written
  [ -e image-dir/inventory.img ]
  # If the inventory.img exists criu checkpointed some things, let's see
  # if there were other errors in the log file.
  run grep -B 5 Error ./work-dir/dump.log -q
  [ "$status" -eq 1 ]

  # This will block until CRIU is ready to serve memory pages
  cat $lazy_pipe
  [ "$status" -eq 1 ]

  unlink $lazy_pipe

  # Double fork taken from helpers.bats
  # We need to start 'criu lazy-pages' in the background,
  # so we double fork in the shell.
  # Start CRIU in lazy-daemon mode
  $(${CRIU} lazy-pages --page-server --address 127.0.0.1 --port ${port} -D image-dir &) &

  # Restore lazily from checkpoint.
  # The restored container needs a different name as the checkpointed
  # container is not yet destroyed. It is only destroyed at that point
  # in time when the last page is lazily transferred to the destination.
  # Killing the CRIU on the checkpoint side will let the container
  # continue to run if the migration failed at some point.
  __runc --criu "$CRIU" restore -d --work-path ./image-dir --image-path ./image-dir --lazy-pages test_busybox_restore <&60 >&51 2>&51
  ret=$?
  [ $ret -eq 0 ]
  run grep -B 5 Error ./work-dir/dump.log -q
  [ "$status" -eq 1 ]

  # busybox should be back up and running
  testcontainer test_busybox_restore running

  runc exec --cwd /bin test_busybox_restore echo ok
  [ "$status" -eq 0 ]
  [[ ${output} == "ok" ]]

  echo Ping >&61
  exec 61>&-
  exec 51>&-
  run cat <&50
  [ "$status" -eq 0 ]
  [[ "${output}" == *"ponG Ping"* ]]
}

@test "checkpoint and restore in external network namespace" {
  # XXX: currently criu require root containers.
  requires criu root

  # check if external_net_ns is supported; only with criu 3.10++
  run ${CRIU} check --feature external_net_ns
  if [ "$status" -eq 1 ]; then
    # this criu does not support external_net_ns; skip the test
    skip "this criu does not support external network namespaces"
  fi

  # create a temporary name for the test network namespace
  tmp=`mktemp`
  rm -f $tmp
  ns_name=`basename $tmp`
  # create network namespace
  ip netns add $ns_name
  ns_path=`ip netns add $ns_name 2>&1 | sed -e 's/.*"\(.*\)".*/\1/'`

  ns_inode=`ls -iL $ns_path | awk '{ print $1 }'`

  # tell runc which network namespace to use
  sed -i "s;\"type\": \"network\";\"type\": \"network\",\"path\": \"$ns_path\";" config.json

  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  testcontainer test_busybox running

  for i in `seq 2`; do
    # checkpoint the running container; this automatically tells CRIU to
    # handle the network namespace defined in config.json as an external
    runc --criu "$CRIU" checkpoint --work-path ./work-dir test_busybox
    ret=$?
    # if you are having problems getting criu to work uncomment the following dump:
    #cat /run/opencontainer/containers/test_busybox/criu.work/dump.log
    cat ./work-dir/dump.log | grep -B 5 Error || true
    [ "$ret" -eq 0 ]

    # after checkpoint busybox is no longer running
    runc state test_busybox
    [ "$status" -ne 0 ]

    # restore from checkpoint; this should restore the container into the existing network namespace
    runc --criu "$CRIU" restore -d --work-path ./work-dir --console-socket $CONSOLE_SOCKET test_busybox
    ret=$?
    cat ./work-dir/restore.log | grep -B 5 Error || true
    [ "$ret" -eq 0 ]

    # busybox should be back up and running
    testcontainer test_busybox running

    # container should be running in same network namespace as before
    pid=`__runc state test_busybox | jq '.pid'`
    ns_inode_new=`readlink /proc/$pid/ns/net | sed -e 's/.*\[\(.*\)\]/\1/'`
    echo "old network namespace inode $ns_inode"
    echo "new network namespace inode $ns_inode_new"
    [ "$ns_inode" -eq "$ns_inode_new" ]
  done
  ip netns del $ns_name
}

@test "checkpoint and restore with container specific CRIU config" {
  # XXX: currently criu require root containers.
  requires criu root

  tmp=`mktemp /tmp/runc-criu-XXXXXX.conf`
  # This is the file we write to /etc/criu/default.conf
  tmplog1=`mktemp /tmp/runc-criu-log-XXXXXX.log`
  unlink $tmplog1
  tmplog1=`basename $tmplog1`
  # That is the actual configuration file to be used
  tmplog2=`mktemp /tmp/runc-criu-log-XXXXXX.log`
  unlink $tmplog2
  tmplog2=`basename $tmplog2`
  # This adds the annotation 'org.criu.config' to set a container
  # specific CRIU config file.
  sed -i "s;\"process\";\"annotations\":{\"org.criu.config\": \"$tmp\"},\"process\";" config.json
  # Tell CRIU to use another configuration file
  mkdir -p /etc/criu
  echo "log-file=$tmplog1" > /etc/criu/default.conf
  # Make sure the RPC defined configuration file overwrites the previous
  echo "log-file=$tmplog2" > $tmp

  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  testcontainer test_busybox running

  # checkpoint the running container
  runc --criu "$CRIU" checkpoint --work-path ./work-dir test_busybox
  [ "$status" -eq 0 ]
  ! test -f ./work-dir/$tmplog1
  test -f ./work-dir/$tmplog2

  # after checkpoint busybox is no longer running
  runc state test_busybox
  [ "$status" -ne 0 ]

  test -f ./work-dir/$tmplog2 && unlink ./work-dir/$tmplog2
  # restore from checkpoint
  runc --criu "$CRIU" restore -d --work-path ./work-dir --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]
  ! test -f ./work-dir/$tmplog1
  test -f ./work-dir/$tmplog2

  # busybox should be back up and running
  testcontainer test_busybox running
  unlink $tmp
  test -f ./work-dir/$tmplog2 && unlink ./work-dir/$tmplog2
}

