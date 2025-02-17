#!/usr/bin/env bash

if [[ "$OSTYPE" = "darwin"* ]]
then
  if go version | grep 'go1.17.[012345]'
  then
    # -race with plug-ins on x86 MacOS 12 with
    # go1.17 before 1.17.6 sometimes crash.
    RACE=
    echo '*** Turning off -race since it may not work on a Mac'
    echo '    with ' `go version`
  fi
fi

ISQUIET=$1
maybe_quiet() {
    if [ "$ISQUIET" == "quiet" ]; then
      "$@" > /dev/null 2>&1
    else
      "$@"
    fi
}

TIMEOUT=timeout
TIMEOUT2=""
if timeout 2s sleep 1 > /dev/null 2>&1
then
  :
else
  if gtimeout 2s sleep 1 > /dev/null 2>&1
  then
    TIMEOUT=gtimeout
  else
    # no timeout command
    TIMEOUT=
    echo '*** Cannot find timeout command; proceeding without timeouts.'
  fi
fi
if [ "$TIMEOUT" != "" ]
then
  TIMEOUT2=$TIMEOUT
  TIMEOUT2+=" -k 2s 120s "
  TIMEOUT+=" -k 2s 45s "
fi

# run the test in a fresh sub-directory.
rm -rf mr-tmp
mkdir mr-tmp || exit 1
cd mr-tmp || exit 1
rm -f mr-*

echo 'building...'

# Build plugins in parallel
(cd ../../mrapps && {
    # Store exit codes in array
    declare -a pids
    go build $RACE -buildmode=plugin wc.go & pids+=($!)
    go build $RACE -buildmode=plugin indexer.go & pids+=($!)
    go build $RACE -buildmode=plugin mtiming.go & pids+=($!)
    go build $RACE -buildmode=plugin rtiming.go & pids+=($!)
    go build $RACE -buildmode=plugin jobcount.go & pids+=($!)
    go build $RACE -buildmode=plugin early_exit.go & pids+=($!)
    go build $RACE -buildmode=plugin crash.go & pids+=($!)
    go build $RACE -buildmode=plugin nocrash.go & pids+=($!)

    # Wait for each process and check its exit status
    for pid in "${pids[@]}"; do
        wait $pid || exit 1
    done
}) || exit 1

# Build main executables in parallel
(cd .. && {
    declare -a pids
    go build $RACE mrcoordinator.go & pids+=($!)
    go build $RACE mrworker.go & pids+=($!)
    go build $RACE mrsequential.go & pids+=($!)

    for pid in "${pids[@]}"; do
        wait $pid || exit 1
    done
}) || exit 1

echo 'finished building'

failed_any=0

#########################################################
echo '***' Starting crash test.

# generate the correct output
../mrsequential ../../mrapps/nocrash.so ../pg*txt || exit 1
sort mr-out-0 > mr-correct-crash.txt
rm -f mr-out*

rm -f mr-done
((maybe_quiet $TIMEOUT2 ../mrcoordinator ../pg*txt); touch mr-done ) &
sleep 1

# start multiple workers
maybe_quiet $TIMEOUT2 ../mrworker ../../mrapps/crash.so &

# mimic rpc.go's coordinatorSock()
SOCKNAME=/var/tmp/5840-mr-`id -u`

( while [ -e $SOCKNAME -a ! -f mr-done ]
  do
    maybe_quiet $TIMEOUT2 ../mrworker ../../mrapps/crash.so
    sleep 1
  done ) &

( while [ -e $SOCKNAME -a ! -f mr-done ]
  do
    maybe_quiet $TIMEOUT2 ../mrworker ../../mrapps/crash.so
    sleep 1
  done ) &

while [ -e $SOCKNAME -a ! -f mr-done ]
do
  maybe_quiet $TIMEOUT2 ../mrworker ../../mrapps/crash.so
  sleep 1
done

wait

rm $SOCKNAME
sort mr-out* | grep . > mr-crash-all
if cmp mr-crash-all mr-correct-crash.txt
then
  echo '---' crash test: PASS
else
  echo '---' crash output is not the same as mr-correct-crash.txt
  echo '---' crash test: FAIL
  failed_any=1
fi
