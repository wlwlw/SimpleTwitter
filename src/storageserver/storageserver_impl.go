package storageserver

import (
	//"errors"
	"sync"
	"fmt"
	"net"
	"net/rpc"
	"net/http"
	"time"
	"log"

	"rpc/storagerpc"
	"util"
)

type storageServer struct {
	nodeID uint32
	ring []uint32
	nodes map[uint32]string
	conns map[string]*rpc.Client
	numNodes int
	storage map[string][]string
	tenants map[string][]string
	// keyLocks map[string]*sync.Mutex
	lock sync.RWMutex
}

// NewStorageServer creates and starts a new StorageServer. masterServerHostPort
// is the master storage server's host:port address. If empty, then this server
// is the master; otherwise, this server is a slave. numNodes is the total number of
// servers in the ring. port is the port number that this server should listen on.
// nodeID is a random, unsigned 32-bit ID identifying this server.
//
// This function should return only once all storage servers have joined the ring,
// and should return a non-nil error if the storage server could not be started.
func NewStorageServer(masterServerHostPort string, numNodes, port int, nodeID uint32) (StorageServer, error) {
	ss := &storageServer{
		nodeID: nodeID,
		ring: make([]uint32, 0),
		nodes: make(map[uint32]string),
		conns: make(map[string]*rpc.Client),
		numNodes: numNodes,
		storage: make(map[string][]string),
		tenants: make(map[string][]string),
		// keyLocks: make(map[string]*sync.Mutex),
	}

	hostport := fmt.Sprintf("localhost:%d", port)
	if masterServerHostPort == "" {
		ss.ring = append(ss.ring, ss.nodeID)
    	ss.nodes[nodeID] = hostport
	}

	listener, err := net.Listen("tcp", hostport)
    if err != nil {
        return nil, err
    }

    err = rpc.RegisterName("StorageServer", storagerpc.Wrap(ss))
    if err != nil {
        return nil, err
    }

	rpc.HandleHTTP()
    go http.Serve(listener, nil)
    
    if masterServerHostPort=="" {
    	for ;len(ss.ring)<numNodes; {
    		time.Sleep(100 * time.Millisecond)
    	}
	} else {
		count := 0
		var client *rpc.Client
		var err error
		for {
			if count>5 {
				log.Fatal("dialing master:", err)
			}
			client, err = rpc.DialHTTP("tcp", masterServerHostPort)
			if err == nil {
				break
			}
			count++
			time.Sleep(1 * time.Second)
		}
		count = 0 
		for {
			args := storagerpc.RegisterArgs{storagerpc.Node{hostport, ss.nodeID}}
			reply := storagerpc.RegisterReply{}
			err = client.Call("StorageServer.RegisterServer", args, &reply)
			if err != nil {
				log.Fatal("rpc error:", err)
			}
			if reply.Status == storagerpc.OK {
				for _, node := range reply.Servers {
					id := node.NodeID
					addr := node.HostPort
					i := util.BinarySearchUint32(ss.ring, id)
					temp := append(ss.ring, 0)
					copy(temp[i+1:], ss.ring[i:])
					temp[i] = id
					ss.ring = temp
					ss.nodes[id] = addr
				}
				ss.numNodes = len(reply.Servers)
				break
			} else if reply.Status == storagerpc.NotReady {
				time.Sleep(1 * time.Second)
			}
			count++
			log.Println("Connect to master retry:", count)
		}
	}
    return ss, nil
}

// func (ss *storageServer) lock(key string) {
// 	ss.locklock.Lock()
// 	lock, ok := ss.keyLocks[key]
// 	if !ok {
// 		lock = new(sync.Mutex)
// 		ss.keyLocks[key] = lock
// 	}
// 	ss.locklock.Unlock()
// 	lock.Lock()
// }

// func (ss *storageServer) unlock(key string) {
// 	ss.locklock.Lock()
// 	lock, ok := ss.keyLocks[key]
// 	if !ok {
// 		log.Fatal("Lose reference to lock of key", key)
// 	}
// 	ss.locklock.Unlock()
// 	lock.Unlock()
// }

func (ss *storageServer) RegisterServer(args *storagerpc.RegisterArgs, reply *storagerpc.RegisterReply) error {
	addr, id := args.ServerInfo.HostPort, args.ServerInfo.NodeID
	ss.lock.Lock()
	defer ss.lock.Unlock()
	if _, ok := ss.nodes[id]; !ok {
		i := util.BinarySearchUint32(ss.ring, id)
		temp := append(ss.ring, 0)
		copy(temp[i+1:], ss.ring[i:])
		temp[i] = id
		ss.ring = temp
		ss.nodes[id] = addr
	}
	if len(ss.ring)>=ss.numNodes {
		reply.Status = storagerpc.OK
		for i, h := range ss.nodes {
			reply.Servers = append(reply.Servers, storagerpc.Node{h, i})
		}
		return nil
	} else {
		reply.Status = storagerpc.NotReady
	}
	return nil
}

func (ss *storageServer) GetServers(args *storagerpc.GetServersArgs, reply *storagerpc.GetServersReply) error {
	if len(ss.ring)>=ss.numNodes {
		reply.Status = storagerpc.OK
		for i, h := range ss.nodes {
			reply.Servers = append(reply.Servers, storagerpc.Node{h, i})
		}
		return nil
	} else {
		reply.Status = storagerpc.NotReady
	}
	return nil
}

func (ss *storageServer) keyRangeContains(key string) bool {
	id := util.SearchHashRing(ss.ring, key)
	if ss.nodeID != id {
		return false
	}
	return true
}

func (ss *storageServer) recordLease(key, hostport string) {
	tlist, exists := ss.tenants[key]
	if !exists {
		tlist = make([]string, 0)
	}
	leaseRecord := util.FormatLeaseRecord(hostport, time.Now().Unix())
	i := util.BinarySearchLeaseRecord(tlist, leaseRecord)
	var tar string
	if i<len(tlist) {
		tar, _ = util.ParseLeaseRecord(tlist[i])
	}
	if i >= len(tlist) || tar!=hostport {
		tlist = append(tlist, "")
		copy(tlist[i+1:], tlist[i:])
		tlist[i]=leaseRecord
		ss.tenants[key]=tlist
	}	
}

func (ss *storageServer) Get(args *storagerpc.GetArgs, reply *storagerpc.GetReply) error {
	key := args.Key
	wantLease := args.WantLease
	if !ss.keyRangeContains(key) {
		reply.Status = storagerpc.WrongServer
		return nil
	}
	ss.lock.Lock()
	defer ss.lock.Unlock()
	if values, ok := ss.storage[key]; ok {
		reply.Status = storagerpc.OK
		if len(values)>0 {
			reply.Value = values[0]
		}
		if wantLease {
			reply.Lease = storagerpc.Lease{true, storagerpc.LeaseSeconds}
			ss.recordLease(key, args.HostPort)
		}
	} else {
		reply.Status = storagerpc.KeyNotFound
	}
	return nil
}

func (ss *storageServer) getAppServer(hostPort string) *rpc.Client {
	cli, ok := ss.conns[hostPort]
	if !ok {
		cli, err := rpc.DialHTTP("tcp", hostPort)
		if err != nil {
			log.Fatal("rpc error:", err)
		}
		ss.conns[hostPort] = cli
		return cli
	}
	return cli
}

func (ss *storageServer) revokeLease(key string) {
	ss.lock.Lock()
	tenants, ok := ss.tenants[key]
	ss.lock.Unlock()
	if !ok {
		return
	}
	ss.lock.Lock()
	for _, leaseRecord := range tenants {
		host, t := util.ParseLeaseRecord(leaseRecord)
		expire_t := t+storagerpc.LeaseSeconds+storagerpc.LeaseGuardSeconds
		if expire_t < time.Now().Unix() {
			continue
		}

		cli := ss.getAppServer(host)
	
		args := &storagerpc.RevokeLeaseArgs{key}
		reply := &storagerpc.RevokeLeaseReply{}
		revokeCall := cli.Go("LeaseCallbacks.RevokeLease", args, reply, nil)
		timeout := time.After(time.Duration(expire_t-time.Now().Unix())*time.Second)
		select {
		case <- timeout:
			continue
		case <- revokeCall.Done:
			continue
		}
	}
	delete(ss.tenants, key)
	ss.lock.Unlock()
}

func (ss *storageServer) Delete(args *storagerpc.DeleteArgs, reply *storagerpc.DeleteReply) error {
	key := args.Key
	if !ss.keyRangeContains(key) {
		reply.Status = storagerpc.WrongServer
		return nil
	}
	ss.lock.Lock()
	_, ok := ss.storage[key]
	ss.lock.Unlock()
	if ok {
		ss.revokeLease(key)
		ss.lock.Lock()
		delete(ss.storage, key)
		ss.lock.Unlock()
		reply.Status = storagerpc.OK
	} else {
		reply.Status = storagerpc.KeyNotFound
	}
	return nil
}

func (ss *storageServer) GetList(args *storagerpc.GetArgs, reply *storagerpc.GetListReply) error {
	key := args.Key
	if !ss.keyRangeContains(key) {
		reply.Status = storagerpc.WrongServer
		return nil
	}
	wantLease := args.WantLease
	ss.lock.Lock()
	defer ss.lock.Unlock()
	if values, ok := ss.storage[key]; ok {
		reply.Status = storagerpc.OK
		if len(values)>0 {
			reply.Value = make([]string, len(values))
			copy(reply.Value, values)
		}
		if wantLease {
			reply.Lease = storagerpc.Lease{true, storagerpc.LeaseSeconds}
			ss.recordLease(key, args.HostPort)
		}
	} else {
		reply.Status = storagerpc.KeyNotFound
	}
	return nil
}

func (ss *storageServer) Put(args *storagerpc.PutArgs, reply *storagerpc.PutReply) error {
	key := args.Key
	if !ss.keyRangeContains(key) {
		reply.Status = storagerpc.WrongServer
		return nil
	}
	
	ss.revokeLease(key)
	ss.lock.Lock()
	ss.storage[key] = []string{args.Value}
	ss.lock.Unlock()
	reply.Status = storagerpc.OK
	return nil
}

func (ss *storageServer) AppendToList(args *storagerpc.PutArgs, reply *storagerpc.PutReply) error {
	key := args.Key
	if !ss.keyRangeContains(key) {
		reply.Status = storagerpc.WrongServer
		return nil
	}
	val := args.Value
	ss.lock.Lock()
	values, ok := ss.storage[key]
	ss.lock.Unlock()
	if ok {
		if len(values) < 1 {
			ss.lock.Lock()
			ss.storage[key] = append(values, val)
			ss.lock.Unlock()
			reply.Status = storagerpc.OK
			return nil
		}
		if values[len(values)-1] < val {
			ss.revokeLease(key)
			ss.lock.Lock()
			ss.storage[key] = append(values, val)
			ss.lock.Unlock()
			reply.Status = storagerpc.OK
			return nil
		}
		
		i := util.BinarySearchString(values, val)
		if values[i]==val {
			reply.Status = storagerpc.ItemExists
			return nil
		}

		ss.revokeLease(key)
		// insert value at index i
		ss.lock.Lock()
		values = append(values, "")
		copy(values[i+1:], values[i:])
		values[i] = val
		ss.storage[key] = values
		ss.lock.Unlock()

		reply.Status = storagerpc.OK
		return nil
	} else {
		ss.lock.Lock()
		ss.storage[key] = []string{val}
		ss.lock.Unlock()
		reply.Status = storagerpc.OK
	}
	return nil
}

func (ss *storageServer) RemoveFromList(args *storagerpc.PutArgs, reply *storagerpc.PutReply) error {
	key := args.Key
	if !ss.keyRangeContains(key) {
		reply.Status = storagerpc.WrongServer
		return nil
	}
	val := args.Value
	ss.lock.Lock()
	values, ok := ss.storage[key]
	ss.lock.Unlock()
	if ok {
		if len(values) < 1 {
			reply.Status = storagerpc.ItemNotFound
			return nil
		}
		ss.lock.Lock()
		mid := util.BinarySearchString(values, val)
		exists := false
		if mid<len(values) && values[mid]==val {
			exists = true
		}
		ss.lock.Unlock()
		if exists {
			// remove value at index mid
			ss.revokeLease(key)
			ss.lock.Lock()
			copy(values[mid:], values[mid+1:])
			ss.storage[key] = values[:len(values)-1]
			ss.lock.Unlock()
			reply.Status = storagerpc.OK
			return nil
		}

		reply.Status = storagerpc.ItemNotFound
		return nil
	} else {
		reply.Status = storagerpc.ItemNotFound
	}
	return nil
}
