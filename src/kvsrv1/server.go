package kvsrv

import (
	"log"

	// "net/rpc"

	"sync"

	"6.5840/kvsrv1/rpc"
	"6.5840/labrpc"
	tester "6.5840/tester1"
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type value struct {
	value   string
	version rpc.Tversion
}

type KVServer struct {
	mu sync.Mutex
	m  map[string]value
}

func MakeKVServer() *KVServer {
	kv := &KVServer{}
	kv.m = make(map[string]value)
	return kv
}

// Get returns the value and version for args.Key, if args.Key
// exists. Otherwise, Get returns ErrNoKey.
func (kv *KVServer) Get(args *rpc.GetArgs, reply *rpc.GetReply) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	v, ok := kv.m[args.Key]
	if !ok {
		reply.Err = rpc.ErrNoKey
		return
	}
	reply.Value = v.value
	reply.Version = v.version
	reply.Err = rpc.OK
}

// Update the value for a key if args.Version matches the version of
// the key on the server. If versions don't match, return ErrVersion.
// If the key doesn't exist, Put installs the value if the
// Args.Version is 0.
func (kv *KVServer) Put(args *rpc.PutArgs, reply *rpc.PutReply) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	v, ok := kv.m[args.Key]
	if ok {
		if args.Version != v.version {
			reply.Err = rpc.ErrVersion
			return
		}
		v.value = args.Value
		v.version++
		kv.m[args.Key] = v
	} else {
		if args.Version != 0 {
			reply.Err = rpc.ErrNoKey
			return
		}
		kv.m[args.Key] = value{
			value:   args.Value,
			version: 1,
		}
	}

	reply.Err = rpc.OK
}

// You can ignore Kill() for this lab
func (kv *KVServer) Kill() {
}

// You can ignore all arguments; they are for replicated KVservers
func StartKVServer(ends []*labrpc.ClientEnd, gid tester.Tgid, srv int, persister *tester.Persister) []tester.IService {
	kv := MakeKVServer()
	return []tester.IService{kv}
}
