package mr

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Coordinator struct {
	mutex          sync.Mutex
	tasks          []task
	workers        []worker
	numMapTasks    int
	numReduceTasks int
	phase          phase
	timeoutSecs    int64
	nReduce        int
}

type task struct {
	files     []string
	status    taskStatus
	startTime int64
	taskType  TaskType
}

type taskStatus int

const (
	taskStatusIdle taskStatus = iota
	taskStatusInProgress
	taskStatusCompleted
	taskStatusTimedOut
	taskStatusFailed
)

type TaskType int

const (
	TaskTypeMap TaskType = iota
	TaskTypeReduce
	TaskTypeNoTask
	TaskTypeShutdown
)

type worker struct {
	taskId    int
	isRunning bool
}

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
	c.createMapTasks(files)
	c.numMapTasks = len(c.tasks)
	c.nReduce = nReduce
	c.phase = phaseMap
	c.timeoutSecs = 10
	c.server()
	return &c
}

func (c *Coordinator) RegisterWorker(req *RegisterWorkerReq, res *RegisterWorkerRes) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	res.WorkerId = len(c.workers)
	c.workers = append(c.workers, worker{isRunning: true})
	return nil
}

func (c *Coordinator) GetTask(req *GetTaskReq, res *GetTaskRes) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	taskId, task := c.findIdleTask()
	if task == nil {
		res.TaskType = TaskTypeNoTask
		c.workers[req.WorkerId].taskId = taskId
		return nil
	}
	task.status = taskStatusInProgress
	task.startTime = time.Now().Unix()
	c.workers[req.WorkerId].taskId = taskId
	res.Files = task.files
	res.NReduce = c.nReduce
	res.TaskType = task.taskType
	res.TaskId = taskId
	return nil
}

func (c *Coordinator) findIdleTask() (int, *task) {
	for i, task := range c.tasks {
		if task.status == taskStatusIdle {
			return i, &c.tasks[i]
		}
	}
	return 0, nil
}

func (c *Coordinator) TaskDone(req *TaskDoneReq, res *TaskDoneRes) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	task := &c.tasks[req.TaskId]

	if task.status == taskStatusTimedOut {
		return nil
	}

	task.status = taskStatusCompleted

	switch task.taskType {
	case TaskTypeMap:
		err := c.renameTempFiles(req.FilesCreated)
		if err != nil {
			return fmt.Errorf("TaskDone renameTempFiles: %w", err)
		}
		c.numMapTasks--
		if c.numMapTasks == 0 {
			err := c.createReduceTasks()
			if err != nil {
				return fmt.Errorf("TaskDone createReduceTasks: %w", err)
			}
			c.phase = phaseReduce
		}
	case TaskTypeReduce:
		err := c.renameTempFiles(req.FilesCreated)
		if err != nil {
			return fmt.Errorf("TaskDone renameTempFiles: %w", err)
		}
		c.numReduceTasks--
		if c.numReduceTasks == 0 {
			c.phase = phaseShutdown
			// create shutdown tasks
			// how to know how many to create?
		}
	}

	return nil
}

func (c *Coordinator) createMapTasks(files []string) {
	c.tasks = make([]task, len(files))
	for i, file := range files {
		c.tasks[i].files = []string{file}
		c.tasks[i].taskType = TaskTypeMap
	}
}

func (c *Coordinator) createReduceTasks() error {
	filenames, err := filepath.Glob("m-out-*-*")
	if err != nil {
		return fmt.Errorf("createReduceTasks filepath.Glob")
	}

	m := make(map[string][]string)

	for _, filename := range filenames {
		// filename format: m-out-<task-id>-<reducer-id>
		parts := strings.Split(filename, "-")
		reduceId := parts[3]
		m[reduceId] = append(m[reduceId], filename)
	}

	for _, files := range m {
		task := task{
			files:    files,
			status:   taskStatusIdle,
			taskType: TaskTypeReduce,
		}
		c.tasks = append(c.tasks, task)
		c.numReduceTasks++
	}

	return nil
}

func (c *Coordinator) renameTempFiles(filesCreated []string) error {
	fmt.Println("renameTempFiles")

	for _, filename := range filesCreated {
		// remove "temp"
		newFilename := strings.Replace(filename, "temp", "", -1)
		fmt.Println("filename:", filename)
		fmt.Println("newFilename:", newFilename)

		err := os.Rename(filename, newFilename)
		if err != nil {
			return fmt.Errorf("failed to rename file %s to %s: %v", filename, newFilename, err)
		}
	}

	return nil
}

// `main/mrcoordinator.go` calls `Done` periodically to check if the entire job has finished
func (c *Coordinator) Done() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.timeoutTasks()

	switch c.phase {
	case phaseMap:
		return false
	case phaseReduce:
		for _, task := range c.tasks {
			if task.status == taskStatusIdle || task.status == taskStatusInProgress {
				return false
			}
		}
	case phaseShutdown:
		for _, worker := range c.workers {
			if worker.isRunning {
				return false
			}
		}
	}

	c.deleteFiles("m-out*")

	return true
}

func (c *Coordinator) timeoutTasks() {
	for i, task_ := range c.tasks {
		if task_.status == taskStatusInProgress && time.Now().Unix() > task_.startTime+c.timeoutSecs {
			c.tasks[i].status = taskStatusTimedOut
			newTask := task{
				files:     task_.files,
				status:    taskStatusIdle,
				startTime: 0,
				taskType:  task_.taskType,
			}
			c.tasks = append(c.tasks, newTask)
			fmt.Println("task timed out:", task_)
			fmt.Println("time.Now().Unix()", time.Now().Unix())
		}
	}
}

func (c *Coordinator) deleteFiles(pattern string) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find files matching pattern %s: %w", pattern, err)
	}
	for _, file := range matches {
		err := os.Remove(file)
		if err != nil {
			return fmt.Errorf("failed to remove file %s: %w", file, err)
		}
	}
	return nil
}

func (c *Coordinator) Heartbeat(req *HeartbeatReq, res *HeartbeatRes) error {
	if c.phase == phaseShutdown {
		res.ShouldShutDown = true
		c.workers[req.WorkerId].isRunning = false
	} else {
		res.ShouldShutDown = false
	}
	return nil
}

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

func (c *Coordinator) printTasks() {
	fmt.Println("tasks ------------------------------")
	for _, task := range c.tasks {
		fmt.Printf("%v\n", task)
	}
	fmt.Println("end tasks ------------------------------")
}
