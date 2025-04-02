package kvsrv

import (
	"time"

	"6.5840/kvsrv1/rpc"
	kvtest "6.5840/kvtest1"
	tester "6.5840/tester1"
)

type Clerk struct {
	clnt   *tester.Clnt
	server string
}

func MakeClerk(clnt *tester.Clnt, server string) kvtest.IKVClerk {
	ck := &Clerk{clnt: clnt, server: server}
	return ck
}

// Get fetches the current value and version for a key.  It returns
// ErrNoKey if the key does not exist. It keeps trying forever in the
// face of all other errors.
func (ck *Clerk) Get(key string) (string, rpc.Tversion, rpc.Err) {
	args := rpc.GetArgs{
		Key: key,
	}

	reply := rpc.GetReply{}

	ok := ck.clnt.Call(ck.server, "KVServer.Get", &args, &reply)

	// retry until success
	for !ok {
		time.Sleep(100 * time.Millisecond)
		ok = ck.clnt.Call(ck.server, "KVServer.Get", &args, &reply)
	}

	if reply.Err != rpc.OK {
		return "", 0, reply.Err
	}

	return reply.Value, reply.Version, rpc.OK
}

// Put updates key with value only if version is the version in the
// request matches the version of the key at the server.  If the
// versions numbers don't match, the server should return
// ErrNoVersion.  If Put receives an ErrVersion on its first RPC, Put
// should return ErrVersion, since the Put was definitely not
// performed at the server. If the server returns ErrVersion on a
// resend RPC, then Put must return ErrMaybe to the application, since
// its earlier RPC might have een processed by the server successfully
// but the response was lost, and the the Clerk doesn't know if
// the Put was performed or not.
func (ck *Clerk) Put(key string, value string, version rpc.Tversion) rpc.Err {
	args := rpc.PutArgs{
		Key:     key,
		Value:   value,
		Version: version,
	}

	reply := rpc.PutReply{}

	ok := ck.clnt.Call(ck.server, "KVServer.Put", &args, &reply)

	// retry until success
	retried := false
	for !ok {
		time.Sleep(100 * time.Millisecond)
		ok = ck.clnt.Call(ck.server, "KVServer.Put", &args, &reply)
		retried = true
	}

	if reply.Err == rpc.ErrVersion && retried {
		return rpc.ErrMaybe
	}

	return reply.Err
}
