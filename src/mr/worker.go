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

	workerId, err := rpcRegisterWorker()
	if err != nil {
		log.Fatal("error in rpcRegisterWorker")
	}

	go heartbeat(workerId)

	for {
		res, err := rpcGetTask(workerId)
		if err != nil {
			// TODO: handle error
			log.Fatal("error in rpcGetTask")
		}

		switch res.TaskType {
		case TaskTypeMap:
			filesCreated, err := runMapTask(res.TaskId, res.Files, res.NReduce, mapf)
			if err != nil {
				// TODO: handle error
				log.Fatal("error in runMapTask")
			}
			_, err = rpcTaskDone(workerId, res.TaskId, filesCreated)
			if err != nil {
				// TODO: handle error
				log.Fatal("error in rpcTaskDone")
			}
		case TaskTypeReduce:
			filesCreated, err := runReduceTask(res.Files, reducef)
			if err != nil {
				// TOOD: handle error
				log.Fatal("error in runReduceTask")
			}
			_, err = rpcTaskDone(workerId, res.TaskId, filesCreated)
			if err != nil {
				// TOOD: handle error
				log.Fatal("error in rpcTaskDone")
			}
		case TaskTypeNoTask:
			time.Sleep(5 * time.Second)
		case TaskTypeShutdown:
			os.Exit(0)
		}
	}
}

func heartbeat(workerId int) {
	for {
		res, err := rpcHeartbeat(workerId)
		if err != nil {
			log.Fatalf("Error in rpcHeartbeat: %v", err)
		}
		if res.ShouldShutDown {
			os.Exit(0)
		}
		time.Sleep(1 * time.Second)
	}
}

func runMapTask(
	taskId int,
	files []string,
	nReduce int,
	mapf func(string, string) []KeyValue,
) ([]string, error) {
	fmt.Println("runMapTask")

	filesCreatedMap := make(map[string]bool)

	for _, inFilename := range files {
		content, err := getFileContent(inFilename)
		if err != nil {
			return []string{}, fmt.Errorf("runMapTask getFileContent(%s): %w", inFilename, err)
		}

		outputKv := mapf(inFilename, content)

		for _, kv := range outputKv {
			// filename format: tempm-out-<task-id>-<reducer-id>
			outFilename := fmt.Sprintf("tempm-out-%d-%d", taskId, ihash(kv.Key)%nReduce)

			filesCreatedMap[outFilename] = true

			// open file and create if it doesn't exist yet
			outFile, err := os.OpenFile(outFilename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				return []string{}, fmt.Errorf("runMapTask OpenFile(%s) %w", outFilename, err)
			}

			fmt.Fprintf(outFile, "%v:::%v\n", kv.Key, kv.Value)

			outFile.Close()
		}
	}

	return mapKeysToSlice(filesCreatedMap), nil
}

func runReduceTask(files []string, reducef func(string, []string) string) ([]string, error) {
	if len(files) == 0 {
		// TODO: handle this?
		return []string{}, nil
	}

	filesMap := make(map[string]bool)

	// filename format: m-out-<task-id>-<reducer-id>
	filenameParts := strings.Split(files[0], "-")
	reduceId := filenameParts[3]

	m := make(map[string][]string)

	for _, filename := range files {
		content, err := getFileContent(filename)
		if err != nil {
			return []string{}, fmt.Errorf("runReduceTask getFileContent(%s): %w", filename, err)
		}

		lines := strings.Split(content, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, ":::")
			key := parts[0]
			value := parts[1]
			m[key] = append(m[key], value)
		}
	}

	for key, values := range m {
		result := reducef(key, values)
		outputFilename := fmt.Sprintf("tempmr-out-%s", reduceId)
		filesMap[outputFilename] = true

		outputFile, err := os.OpenFile(outputFilename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			return []string{}, fmt.Errorf("runReduceTask OpenFile(%s) %w", outputFilename, err)
		}

		fmt.Fprintf(outputFile, "%s %s\n", key, result)
		outputFile.Close()
	}

	return mapKeysToSlice(filesMap), nil
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

func mapKeysToSlice[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// to choose the reduce task number for each `KeyValue` emitted by `Map`, do `ihash(key) % NReduce`
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

func rpcRegisterWorker() (int, error) {
	req := RegisterWorkerReq{}
	res := RegisterWorkerRes{}

	fmt.Printf("rpcRegisterWorker req: %v\n", req)

	ok := call("Coordinator.RegisterWorker", &req, &res)
	if !ok {
		return 0, fmt.Errorf("rpcGetTask failed")
	}

	fmt.Printf("rpcRegisterWorker res: %v\n", res)

	return res.WorkerId, nil
}

func rpcGetTask(workerId int) (GetTaskRes, error) {
	req := GetTaskReq{
		WorkerId: workerId,
	}
	res := GetTaskRes{}

	fmt.Printf("rpcGetTask req: %v\n", req)

	ok := call("Coordinator.GetTask", &req, &res)
	if !ok {
		return GetTaskRes{}, fmt.Errorf("rpcGetTask failed")
	}

	fmt.Printf("rpcGetTask res: %v\n", res)

	return res, nil
}

func rpcTaskDone(workerId int, taskId int, filesCreated []string) (TaskDoneRes, error) {
	req := TaskDoneReq{
		WorkerId:     workerId,
		TaskId:       taskId,
		FilesCreated: filesCreated,
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

func rpcHeartbeat(workerId int) (HeartbeatRes, error) {
	req := HeartbeatReq{
		WorkerId: workerId,
	}
	res := HeartbeatRes{}
	ok := call("Coordinator.Heartbeat", &req, &res)
	if !ok {
		return HeartbeatRes{}, fmt.Errorf("rpcHeartbeat failed")
	}
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
