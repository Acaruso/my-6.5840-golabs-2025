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
# first word-count

# generate the correct output
../mrsequential ../../mrapps/wc.so ../pg*txt || exit 1
sort mr-out-0 > mr-correct-wc.txt
rm -f mr-out*

echo '***' Starting wc test.

maybe_quiet $TIMEOUT ../mrcoordinator ../pg*txt &
pid=$!

# give the coordinator time to create the sockets.
sleep 1

# start multiple workers.
(maybe_quiet $TIMEOUT ../mrworker ../../mrapps/wc.so) &
(maybe_quiet $TIMEOUT ../mrworker ../../mrapps/wc.so) &
(maybe_quiet $TIMEOUT ../mrworker ../../mrapps/wc.so) &

# wait for the coordinator to exit.
wait $pid

# since workers are required to exit when a job is completely finished,
# and not before, that means the job has finished.
sort mr-out* | grep . > mr-wc-all
if cmp mr-wc-all mr-correct-wc.txt
then
  echo '---' wc test: PASS
else
  echo '---' wc output is not the same as mr-correct-wc.txt
  echo '---' wc test: FAIL
  failed_any=1
fi

# wait for remaining workers and coordinator to exit.
wait
