package mr

import (
	"fmt"
	"os"
	"strconv"
)

type GetTaskReq struct {
}

type GetTaskRes struct {
	Filename string
	NReduce  int
	TaskType TaskType
}

// type TaskType int

// const (
// 	TaskTypeMap TaskType = iota
// 	TaskTypeReduce
// 	TaskTypeNoJob
// )

func (t *GetTaskRes) String() string {
	return fmt.Sprintf("Filename: %s, NReduce: %d, TaskType: %d", t.Filename, t.NReduce, t.TaskType)
}

type TaskDoneReq struct {
	Filename string
}

type TaskDoneRes struct {
}

// create a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
