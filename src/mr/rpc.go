package mr

// RPC definitions
// remember to capitalize all names

import (
	"os"
	"strconv"
)

// example of how to declare the argument and reply type for an RPC method
type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// add your RPC definitions here

// create a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
