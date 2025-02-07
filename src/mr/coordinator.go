package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
)

// create a `Coordinator`
// this is called by `main/mrcoordinator.go`
// `nReduce` is the number of reduce tasks to use
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{}

	// your code here

	c.server()
	return &c
}

type Coordinator struct {
	// your definitions here
}

// an example RPC handler
// the RPC argument and reply types are defined in `rpc.go`
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
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

// main/mrcoordinator.go calls `Done` periodically to check if the entire job has finished
func (c *Coordinator) Done() bool {
	ret := false

	// your code here

	return ret
}
