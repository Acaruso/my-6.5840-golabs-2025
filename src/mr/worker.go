package mr

import (
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"strings"
	"time"
)

type KeyValue struct {
	Key   string
	Value string
}

// this is the entrypoint worker function
// `main/mrworker.go` calls this
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	for {
		res, err := rpcGetTask()
		if err != nil {
			// TODO: handle error
		}

		switch res.TaskType {
		case TaskTypeMap:
			err := runMapTask(res.Filename, res.NReduce, mapf)
			if err != nil {
				// TODO: handle error
			}
			_, err = rpcTaskDone(res.Filename)
			if err != nil {
				// TODO: handle error
			}
		case TaskTypeReduce:
			err = runReduceTask(res.Filename, reducef)
			if err != nil {
				// TOOD: handle error
			}
			_, err = rpcTaskDone(res.Filename)
			if err != nil {
				// TOOD: handle error
			}
		case TaskTypeNoJob:
			time.Sleep(5 * time.Second)
		case TaskTypeShutdown:
			os.Exit(0)
		}
	}
}

func runMapTask(filename string, nReduce int, mapf func(string, string) []KeyValue) error {
	fmt.Println("runMapTask")

	content, err := getFileContent(filename)
	if err != nil {
		return fmt.Errorf("runMapTask getFileContent(%s): %w", filename, err)
	}

	outputKv := mapf(filename, content)

	for _, kv := range outputKv {
		outputFilename := fmt.Sprintf("map-output-%d", ihash(kv.Key)%nReduce)

		// open file and create if it doesn't exist yet
		// opening with `O_APPEND` makes small writes atomic
		outputFile, err := os.OpenFile(outputFilename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			return fmt.Errorf("runMapTask OpenFile(%s) %w", outputFilename, err)
		}

		fmt.Fprintf(outputFile, "%v %v\n", kv.Key, kv.Value)

		outputFile.Close()
	}

	return nil
}

func runReduceTask(filename string, reducef func(string, []string) string) error {
	filenameParts := strings.Split(filename, "-")
	taskNum := filenameParts[2]

	content, err := getFileContent(filename)
	if err != nil {
		return fmt.Errorf("runReduceTask getFileContent(%s): %w", filename, err)
	}

	lines := strings.Split(content, "\n")

	m := make(map[string][]string)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		key := parts[0]
		value := parts[1]
		m[key] = append(m[key], value)
	}

	for key, values := range m {
		result := reducef(key, values)

		outputFilename := fmt.Sprintf("mr-out-%s", taskNum)
		outputFile, err := os.OpenFile(outputFilename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			return fmt.Errorf("runReduceTask OpenFile(%s) %w", outputFilename, err)
		}

		fmt.Fprintf(outputFile, "%s %s\n", key, result)
		outputFile.Close()
	}

	return nil
}

func getFileContent(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("getFileContent error opening file %w", err)
	}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("getFileContent error reading file %w", err)
	}

	return string(content), nil
}

// to choose the reduce task number for each `KeyValue` emitted by `Map`, do `ihash(key) % NReduce`
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

func rpcGetTask() (GetTaskRes, error) {
	req := GetTaskReq{}
	res := GetTaskRes{}

	fmt.Printf("rpcGetTask req: %v\n", req)

	ok := call("Coordinator.GetTask", &req, &res)
	if !ok {
		return GetTaskRes{}, fmt.Errorf("rpcGetTask failed")
	}

	fmt.Printf("rpcGetTask res: %v\n", res)

	return res, nil
}

func rpcTaskDone(fileName string) (TaskDoneRes, error) {
	req := TaskDoneReq{
		Filename: fileName,
	}
	res := TaskDoneRes{}

	fmt.Printf("rpcTaskDone req: %v\n", req)

	ok := call("Coordinator.TaskDone", &req, &res)
	if !ok {
		return TaskDoneRes{}, fmt.Errorf("rpcTaskDone failed")
	}

	fmt.Printf("rpcTaskDone res: %v\n", res)

	return res, nil
}

// send an RPC request to the coordinator, wait for the response
// usually returns `true`
// returns `false` if something goes wrong
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
