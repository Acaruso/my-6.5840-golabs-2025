package mr

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type RegisterWorkerReq struct {
}

type RegisterWorkerRes struct {
	WorkerId int
}

func (r RegisterWorkerRes) String() string {
	return fmt.Sprintf("{WorkerId: %d}", r.WorkerId)
}

type GetTaskReq struct {
	WorkerId int
}

type GetTaskRes struct {
	Files    []string
	NReduce  int
	TaskType TaskType
	TaskId   int
}

func (r GetTaskReq) String() string {
	return fmt.Sprintf("{WorkerId: %d}", r.WorkerId)
}

func (r GetTaskRes) String() string {
	return fmt.Sprintf(
		"{Files: [%s], NReduce: %d, TaskType: %d, TaskId: %d}",
		strings.Join(r.Files, ", "),
		r.NReduce,
		r.TaskType,
		r.TaskId)
}

type TaskDoneReq struct {
	WorkerId     int
	TaskId       int
	FilesCreated []string
}

type TaskDoneRes struct {
}

func (r TaskDoneReq) String() string {
	return fmt.Sprintf(
		"{WorkerId: %d, TaskId: %d, FilesCreated: [%s]}",
		r.WorkerId,
		r.TaskId,
		strings.Join(r.FilesCreated, ", "))
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
