package libstore

import (
	"errors"
	"log"
	"net/rpc"
	"sync"
	"time"

	"rpc/librpc"
	"rpc/storagerpc"
)

type record struct {
	storagerpc.Lease
	history []int64
}

type libstore struct {
	hostPort string
	mode     LeaseMode
	ring     []uint32
	nodes    map[uint32]string
	conns    map[uint32]*rpc.Client
	cache    map[string][]string
	records  map[string]*record
	// keyLocks map[string]*sync.Mutex
	lock     sync.Mutex
}

func binarySearchUint32(vals []uint32, val uint32) int {
	i, j, mid := 0, len(vals)-1, 0
	for i <= j {
		mid = (i + j) / 2
		if vals[mid] == val {
			return mid
		} else if vals[mid] < val {
			i = mid + 1
		} else {
			j = mid - 1
		}
	}
	return i
}

func searchHashRing(ring []uint32, key string) uint32 {
	hash := StoreHash(key)
	i := binarySearchUint32(ring, hash)
	if i == len(ring) {
		i = 0
	}
	return ring[i]
}

// NewLibstore creates a new instance of a TribServer's libstore. masterServerHostPort
// is the master storage server's host:port. myHostPort is this Libstore's host:port
// (i.e. the callback address that the storage servers should use to send back
// notifications when leases are revoked).
//
// The mode argument is a debugging flag that determines how the Libstore should
// request/handle leases. If mode is Never, then the Libstore should never request
// leases from the storage server (i.e. the GetArgs.WantLease field should always
// be set to false). If mode is Always, then the Libstore should always request
// leases from the storage server (i.e. the GetArgs.WantLease field should always
// be set to true). If mode is Normal, then the Libstore should make its own
// decisions on whether or not a lease should be requested from the storage server,
// based on the requirements specified in the project PDF handout.  Note that the
// value of the mode flag may also determine whether or not the Libstore should
// register to receive RPCs from the storage servers.
//
// To register the Libstore to receive RPCs from the storage servers, the following
// line of code should suffice:
//
//     rpc.RegisterName("LeaseCallbacks", librpc.Wrap(libstore))
//
// Note that unlike in the NewTribServer and NewStorageServer functions, there is no
// need to create a brand new HTTP handler to serve the requests (the Libstore may
// simply reuse the TribServer's HTTP handler since the two run in the same process).
func NewLibstore(masterServerHostPort, myHostPort string, mode LeaseMode) (Libstore, error) {
	ls := &libstore{
		hostPort: myHostPort,
		mode:     mode,
		ring:     make([]uint32, 0),
		nodes:    make(map[uint32]string),
		conns:    make(map[uint32]*rpc.Client),
		cache:    make(map[string][]string),
		records:  make(map[string]*record),
		// keyLocks: make(map[string]*sync.Mutex),
	}

	client, err := rpc.DialHTTP("tcp", masterServerHostPort)
	if err != nil {
		return nil, err
	}
	for re := 0; ; re++ {
		if re >= 5 {
			return nil, errors.New("Timeout when waiting StorageServer ready.")
		}
		args := storagerpc.GetServersArgs{}
		reply := storagerpc.GetServersReply{}
		err = client.Call("StorageServer.GetServers", args, &reply)
		if err != nil {
			log.Fatal("rpc error:", err)
		}
		if reply.Status == storagerpc.OK {
			for _, node := range reply.Servers {
				id := node.NodeID
				addr := node.HostPort
				i := binarySearchUint32(ls.ring, id)
				temp := append(ls.ring, 0)
				copy(temp[i+1:], ls.ring[i:])
				temp[i] = id
				ls.ring = temp
				ls.nodes[id] = addr
			}
			break
		} else if reply.Status == storagerpc.NotReady {
			time.Sleep(1 * time.Second)
		}
	}

	if mode != Never && myHostPort != "" {
		err = rpc.RegisterName("LeaseCallbacks", librpc.Wrap(ls))
		if err != nil {
			return nil, err
		}
	}
	go ls.cacheRecycler()
	return ls, nil
}

// func (ls *libstore) lockKey(key string) {
// 	ls.lock.Lock()
// 	l, ok := ls.keyLocks[key]
// 	if !ok {
// 		l = new(sync.Mutex)
// 		ls.keyLocks[key] = l
// 	}
// 	ls.lock.Unlock()
// 	l.Lock()
// }

// func (ls *libstore) unlockKey(key string) {
// 	ls.lock.Lock()
// 	l, ok := ls.keyLocks[key]
// 	if !ok {
// 		log.Fatal("Lose reference to lock of key", key)
// 	}
// 	ls.lock.Unlock()
// 	l.Unlock()
// }

func (ls *libstore) cacheRecycler() {
	for {
		if len(ls.cache)==0 {
			time.Sleep(time.Duration(storagerpc.LeaseSeconds)*time.Second)
			continue
		}
		for key := range ls.cache {
			ls.lock.Lock()
			rec, ok := ls.records[key] 
			if ok && (rec.history[len(rec.history)-1] < time.Now().Unix()-int64(rec.ValidSeconds) || rec.Granted == false) {
				delete(ls.cache, key)
				delete(ls.records, key)
			}
			ls.lock.Unlock()
			var l int
			if len(ls.cache) > 0 {
				l = len(ls.cache)
			} else {
				l = 1
			}
			time.Sleep(time.Duration(storagerpc.LeaseSeconds/l)*time.Second)
		}
	}
}

func (ls *libstore) getStorageServer(key string) *rpc.Client {
	id := searchHashRing(ls.ring, key)
	cli, ok := ls.conns[id]
	if !ok {
		cli, err := rpc.DialHTTP("tcp", ls.nodes[id])
		if err != nil {
			log.Fatal("rpc error:", err)
		}
		ls.conns[id] = cli
		return cli
	}
	return cli
}

func (ls *libstore) Get(key string) (string, error) {
	ls.lock.Lock()
	wantLease := false
	rec, recExists := ls.records[key]
	switch ls.mode {
	case Never:
		wantLease = false
	case Always:
		wantLease = true
	default:
		if recExists {
			if rec.history[len(rec.history)-1] < time.Now().Unix()-int64(rec.ValidSeconds) {
				rec.Granted = false
				// delete(ls.cache, key)
			}
			if rec.Granted {
				rec.history = append(rec.history, time.Now().Unix())
				if len(rec.history) > storagerpc.QueryCacheThresh {
					rec.history = rec.history[len(rec.history)-storagerpc.QueryCacheThresh:]
				}
				values, ok := ls.cache[key]
				if !ok {
					log.Fatal("Cache inconsistent at key", key)
				}
				result := make([]string, len(values))
				copy(result, values)
				ls.lock.Unlock()
				return result[0], nil
			} else {
				if len(rec.history) >= storagerpc.QueryCacheThresh && rec.history[0] > time.Now().Unix()-int64(storagerpc.QueryCacheSeconds) {
					wantLease = true
				}
			}
		}
	}
	ls.lock.Unlock()

	// not cached retrieve from remote server
	cli := ls.getStorageServer(key)
	args := &storagerpc.GetArgs{Key: key, WantLease: wantLease, HostPort: ls.hostPort}
	var reply storagerpc.GetReply
	t := time.Now()
	err := cli.Call("StorageServer.Get", args, &reply)
	if time.Now().Sub(t)>100*time.Millisecond {
		log.Printf("Slow StorageServer.Get")
	}
	if err != nil {
		return "", err
	}
	if reply.Status == storagerpc.OK {
		ls.lock.Lock()
		if !recExists {
			rec = &record{
				storagerpc.Lease{false, storagerpc.LeaseSeconds},
				make([]int64, 0),
			}
			ls.records[key] = rec
		}
		rec.history = append(rec.history, time.Now().Unix())
		if len(rec.history) > storagerpc.QueryCacheThresh {
			rec.history = rec.history[len(rec.history)-storagerpc.QueryCacheThresh:]
		}
		if wantLease {
			rec.Granted = reply.Lease.Granted
			rec.ValidSeconds = reply.Lease.ValidSeconds
			ls.cache[key] = []string{reply.Value}
		}
		ls.lock.Unlock()
		return reply.Value, nil
	} else if reply.Status == storagerpc.KeyNotFound {
		return "", errors.New("Key " + key + " not found")
	}
	return "", errors.New("Should not be reached")
}

func (ls *libstore) Put(key, value string) error {
	cli := ls.getStorageServer(key)
	args := storagerpc.PutArgs{key, value}
	reply := storagerpc.PutReply{}

	t := time.Now()
	err := cli.Call("StorageServer.Put", &args, &reply)
	if time.Now().Sub(t)>100*time.Millisecond {
		log.Println("Slow StorageServer.Put")
	}

	if err != nil {
		return err
	}
	if reply.Status == storagerpc.OK {
		return nil
	}
	return errors.New("Should not be reached")
}

func (ls *libstore) Delete(key string) error {
	cli := ls.getStorageServer(key)
	args := storagerpc.DeleteArgs{key}
	reply := storagerpc.DeleteReply{}

	t := time.Now()
	err := cli.Call("StorageServer.Delete", &args, &reply)
	if time.Now().Sub(t)>100*time.Millisecond {
		log.Println("Slow StorageServer.Delete")
	}

	if err != nil {
		return err
	}
	if reply.Status == storagerpc.OK {
		return nil
	} else if reply.Status == storagerpc.KeyNotFound {
		return errors.New("Key " + key + " not found")
	}
	return errors.New("Should not be reached")
}

func (ls *libstore) GetList(key string) ([]string, error) {
	ls.lock.Lock()
	wantLease := false
	rec, recExists := ls.records[key]
	switch ls.mode {
	case Never:
		wantLease = false
	case Always:
		wantLease = true
	default:
		if recExists {
			if rec.history[len(rec.history)-1] < time.Now().Unix()-int64(rec.ValidSeconds) {
				rec.Granted = false
				// delete(ls.cache, key)
			}
			if rec.Granted {
				rec.history = append(rec.history, time.Now().Unix())
				if len(rec.history) > storagerpc.QueryCacheThresh {
					rec.history = rec.history[len(rec.history)-storagerpc.QueryCacheThresh:]
				}

				values, ok := ls.cache[key]
				if !ok {
					log.Fatal("Cache inconsistent at key", key)
				}

				result := make([]string, len(values))
				copy(result, values)
				ls.lock.Unlock()
				return result, nil
			} else {
				if len(rec.history) >= storagerpc.QueryCacheThresh && rec.history[0] > time.Now().Unix()-int64(storagerpc.QueryCacheSeconds) {
					wantLease = true
				}
			}
		}
	}
	ls.lock.Unlock()
	// not cached retrieve from remote server
	cli := ls.getStorageServer(key)
	

	args := storagerpc.GetArgs{key, wantLease, ls.hostPort}
	reply := storagerpc.GetListReply{}

	t := time.Now()
	err := cli.Call("StorageServer.GetList", &args, &reply)
	if time.Now().Sub(t)>100*time.Millisecond {
		log.Println("Slow StorageServer.GetList")
	}

	if err != nil {
		return nil, err
	}
	if reply.Status == storagerpc.OK {
		ls.lock.Lock()
		if !recExists {
			rec = &record{
				storagerpc.Lease{false, storagerpc.LeaseSeconds},
				make([]int64, 0),
			}
			ls.records[key] = rec
		}
		rec.history = append(rec.history, time.Now().Unix())
		if len(rec.history) > storagerpc.QueryCacheThresh {
			rec.history = rec.history[len(rec.history)-storagerpc.QueryCacheThresh:]
		}
		if wantLease {
			rec.Granted = reply.Lease.Granted
			rec.ValidSeconds = reply.Lease.ValidSeconds
			ls.cache[key] = reply.Value
		}
		result := make([]string, len(reply.Value))
		copy(result, reply.Value)
		ls.lock.Unlock()
		return result, nil
	} else if reply.Status == storagerpc.KeyNotFound {
		return nil, errors.New("Key " + key + " not found")
	}
	return nil, errors.New("Unknown reply")
}

func (ls *libstore) RemoveFromList(key, removeItem string) error {
	cli := ls.getStorageServer(key)
	args := storagerpc.PutArgs{key, removeItem}
	reply := storagerpc.PutReply{}
	t := time.Now()
	err := cli.Call("StorageServer.RemoveFromList", &args, &reply)
	if time.Now().Sub(t)>100*time.Millisecond {
		log.Println("Slow StorageServer.RemoveFromList")
	}
	if err != nil {
		return err
	}
	if reply.Status == storagerpc.OK {
		return nil
	} else if reply.Status == storagerpc.ItemNotFound {
		return errors.New("Item " + removeItem + " not found under key " + key)
	}
	return errors.New("Unknown reply")
}

func (ls *libstore) AppendToList(key, newItem string) error {
	cli := ls.getStorageServer(key)
	args := storagerpc.PutArgs{key, newItem}
	reply := storagerpc.PutReply{}
	t := time.Now()
	err := cli.Call("StorageServer.AppendToList", &args, &reply)
	if time.Now().Sub(t)>100*time.Millisecond {
		log.Println("Slow StorageServer.AppendToList")
	}
	if err != nil {
		return err
	}
	if reply.Status == storagerpc.OK {
		return nil
	} else if reply.Status == storagerpc.ItemExists {
		return errors.New("Item " + newItem + " exists under key " + key)
	}
	return errors.New("Unknown reply")
}

func (ls *libstore) RevokeLease(args *storagerpc.RevokeLeaseArgs, reply *storagerpc.RevokeLeaseReply) error {
	key := args.Key
	ls.lock.Lock()
	defer ls.lock.Unlock()
	if _, ok := ls.cache[key]; ok {
		delete(ls.cache, key)
		ls.records[key].Granted = false
		reply.Status = storagerpc.OK
	} else {
		reply.Status = storagerpc.KeyNotFound
	}
	return nil
}
