package lock

import (
	"time"

	"6.5840/kvsrv1/rpc"
	kvtest "6.5840/kvtest1"
)

type Lock struct {
	// IKVClerk is a go interface for k/v clerks: the interfaces hides
	// the specific Clerk type of ck but promises that ck supports
	// Put and Get.  The tester passes the clerk in when calling
	// MakeLock().
	ck      kvtest.IKVClerk
	lockKey string
	version rpc.Tversion
}

// The tester calls MakeLock() and passes in a k/v clerk; you code can
// perform a Put or Get by calling lk.ck.Put() or lk.ck.Get().
func MakeLock(ck kvtest.IKVClerk, l string) *Lock {
	lk := &Lock{ck: ck}
	lk.lockKey = l
	lk.version = 0
	_, _, err := lk.ck.Get(lk.lockKey)
	if err == rpc.ErrNoKey {
		err = lk.ck.Put(lk.lockKey, "0", lk.version)
		if err == rpc.OK {
			lk.version++
		}
	}
	return lk
}

func (lk *Lock) Acquire() {
	for !lk.tryAcquire() {
		time.Sleep(1 * time.Second)
	}
}

func (lk *Lock) tryAcquire() bool {
	val := ""
	var version rpc.Tversion = 0
	var err rpc.Err = rpc.OK

	val, version, err = lk.ck.Get(lk.lockKey)
	if err != rpc.OK {
		return false
	}

	if val == "1" {
		return false
	}

	lk.version = version

	err = lk.ck.Put(lk.lockKey, "1", lk.version)
	switch err {
	case rpc.OK:
		lk.version++
		return true
	case rpc.ErrVersion:
		return false
	case rpc.ErrNoKey:
		// should never happen
		return false
	default:
		return false
	}
}

func (lk *Lock) Release() {
	err := lk.ck.Put(lk.lockKey, "0", lk.version)
	if err != rpc.OK {
		// idk how to handle this
	}
	lk.version++
}
