package mr

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Coordinator struct {
	mutex          sync.Mutex
	tasks          []task
	numMapTasks    int
	numReduceTasks int
	phase          phase
	timeoutSecs    int64
	nReduce        int
}

type task struct {
	filename  string
	status    taskStatus
	startTime int64
	taskType  TaskType
}

type taskStatus int

const (
	taskStatusIdle taskStatus = iota
	taskStatusInProgress
	taskStatusCompleted
	taskStatusFailed
)

type TaskType int

const (
	TaskTypeMap TaskType = iota
	TaskTypeReduce
	TaskTypeNoJob
	TaskTypeShutdown
)

type phase int

const (
	phaseMap phase = iota
	phaseReduce
	phaseShutdown
)

// create a `Coordinator`
// this is called by `main/mrcoordinator.go`
// `nReduce` is the number of reduce tasks to use
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{}
	c.tasks = make([]task, len(files))
	for i, file := range files {
		c.tasks[i].filename = file
		c.tasks[i].taskType = TaskTypeMap
	}
	c.nReduce = nReduce
	c.phase = phaseMap
	c.timeoutSecs = 10
	c.numMapTasks = len(c.tasks)
	c.server()
	return &c
}

// func (c *Coordinator) RegisterWorker(req *GetTaskReq, res *GetTaskRes) error {
// }

func (c *Coordinator) GetTask(req *GetTaskReq, res *GetTaskRes) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	task := c.findIdleTask()
	if task == nil {
		res.TaskType = TaskTypeNoJob
		return nil
	}
	task.status = taskStatusInProgress
	task.startTime = time.Now().Unix()
	res.Filename = task.filename
	res.NReduce = c.nReduce
	if task.taskType == TaskTypeMap {
		res.TaskType = TaskTypeMap
	} else if task.taskType == TaskTypeReduce {
		res.TaskType = TaskTypeReduce
	}
	return nil
}

func (c *Coordinator) findIdleTask() *task {
	for i, task := range c.tasks {
		if task.status == taskStatusIdle {
			return &c.tasks[i]
		}
	}
	return nil
}

func (c *Coordinator) TaskDone(req *TaskDoneReq, res *TaskDoneRes) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	task := c.findTask(req.Filename)
	if task == nil {
		return nil
	}

	task.status = taskStatusCompleted

	switch task.taskType {
	case TaskTypeMap:
		c.numMapTasks--
		if c.numMapTasks == 0 {
			err := c.createReduceJobs()
			if err != nil {
				return fmt.Errorf("TaskDone createReduceJobs: %w", err)
			}
			c.phase = phaseReduce
		}
	case TaskTypeReduce:
		c.numReduceTasks--
		if c.numReduceTasks == 0 {
			c.phase = phaseShutdown
			// create shutdown jobs
			// how to know how many to create?
		}
	}

	return nil
}

func (c *Coordinator) findTask(fileName string) *task {
	for i, job := range c.tasks {
		if job.filename == fileName {
			return &c.tasks[i]
		}
	}
	return nil
}

func (c *Coordinator) createReduceJobs() error {
	files, err := filepath.Glob("map-output-*")
	if err != nil {
		return fmt.Errorf("createReduceJobs filepath.Glob(\"map-output-*\")")
	}

	for _, file := range files {
		task := task{
			filename:  file,
			status:    taskStatusIdle,
			startTime: time.Now().Unix(),
			taskType:  TaskTypeReduce,
		}
		c.tasks = append(c.tasks, task)
		c.numReduceTasks++
	}

	return nil
}

// `main/mrcoordinator.go` calls `Done` periodically to check if the entire job has finished
func (c *Coordinator) Done() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.phase == phaseMap {
		return false
	}

	for _, task := range c.tasks {
		if task.status == taskStatusIdle || task.status == taskStatusInProgress {
			return false
		}
	}

	return true
}

// func (c *Coordinator) reAssignJobs() {
// 	for i, job := range c.jobs {
//         if job.status == inProgress && (time.Now().Unix() - job.startTime >= c.timeoutSecs) {
//             newJob := job
//             newJob.status = idle
//             c.jobs = append(c.jobs, newJob)
//         }
// 	}
// }

// start a thread that listens for RPCs from `worker.go`
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	// l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}
