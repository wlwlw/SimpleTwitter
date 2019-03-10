package stwserver

import (
	//"errors"
	"net"
	"net/rpc"
	"net/http"
	"time"
	//"math"
	"sort"
	"log"
	"strconv"

	"rpc/stwrpc"
	"libstore"
	"util"
)

type stwServer struct {
	nodes []string
	numNodes int
	storage libstore.Libstore
}

func NewStwServer(myHostPort, masterServer, masterStorageServer string, numNodes int) (StwServer, error) {
    ts := &stwServer{
    	nodes: []string{myHostPort},
    	numNodes: numNodes,
    }

    storage, err := libstore.NewLibstore(
    	masterStorageServer, myHostPort, libstore.Normal,
    )
    if err != nil {
		return nil, err
	}
	ts.storage = storage

    listener, err := net.Listen("tcp", myHostPort)
    if err != nil {
        return nil, err
    }

    // Wrap the stwServer before registering it for RPC.
    err = rpc.RegisterName("StwServer", stwrpc.Wrap(ts))
    if err != nil {
        return nil, err
    }

    rpc.HandleHTTP()
    go http.Serve(listener, nil)


 	// forming cluster
 	if masterServer=="" {
    	for ;len(ts.nodes)<numNodes; {
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
			client, err = rpc.DialHTTP("tcp", masterServer)
			if err == nil {
				break
			}
			count++
			time.Sleep(1 * time.Second)
		}
		count = 0 
		for {
			args := stwrpc.RegisterArgs{stwrpc.Node{myHostPort}}
			reply := stwrpc.RegisterReply{}
			err = client.Call("StwServer.RegisterServer", args, &reply)
			if err != nil {
				log.Fatal("rpc error:", err)
			}
			if reply.Status == stwrpc.OK {
				for _, node := range reply.Servers {
					addr := node.HostPort
					i := util.BinarySearchString(ts.nodes, addr)
					temp := append(ts.nodes, "")
					copy(temp[i+1:], ts.nodes[i:])
					ts.nodes = temp
				}
				ts.numNodes = len(reply.Servers)
				break
			} else if reply.Status == stwrpc.NotReady {
				time.Sleep(1 * time.Second)
			}
			count++
			log.Println("Connect to master retry:", count)
		}
	}

    return ts, nil
}


func (ts *stwServer) RegisterServer(args *stwrpc.RegisterArgs, reply *stwrpc.RegisterReply) error {
	addr := args.ServerInfo.HostPort
	i := util.BinarySearchString(ts.nodes, addr)
	if i >= len(ts.nodes) || ts.nodes[i]!=addr {
		temp := append(ts.nodes, "")
		copy(temp[i+1:], ts.nodes[i:])
		temp[i] = addr
		ts.nodes = temp
	}
	if len(ts.nodes)>=ts.numNodes {
		reply.Status = stwrpc.OK
		for _, h := range ts.nodes {
			reply.Servers = append(reply.Servers, stwrpc.Node{h})
		}
		return nil
	} else {
		reply.Status = stwrpc.NotReady
	}
	return nil
}

func (ts *stwServer) GetServers(args *stwrpc.GetServersArgs, reply *stwrpc.GetServersReply) error {
	if len(ts.nodes)>=ts.numNodes {
		reply.Status = stwrpc.OK
		for _, h := range ts.nodes {
			reply.Servers = append(reply.Servers, stwrpc.Node{h})
		}
		return nil
	} else {
		reply.Status = stwrpc.NotReady
	}
	return nil
}

func (ts *stwServer) CreateUser(args *stwrpc.CreateUserArgs, reply *stwrpc.CreateUserReply) error {
	key := util.FormatUserKey(args.UserID)
	_, err := ts.storage.Get(key)
	if err == nil {
		reply.Status = stwrpc.Exists
		return nil
	}
	ts.storage.Put(key, "")
	reply.Status = stwrpc.OK
	return nil
}

func (ts *stwServer) Subscribe(args *stwrpc.SubscriptionArgs, reply *stwrpc.SubscriptionReply) error {
	sKey := util.FormatUserKey(args.UserID)
	tKey := util.FormatUserKey(args.TargetUserID)
	slistKey := util.FormatSubListKey(args.UserID)
	_, err := ts.storage.Get(sKey)
	if err != nil {
		reply.Status = stwrpc.NoSuchUser
		return nil
	}
	_, err = ts.storage.Get(tKey)
	if err != nil {
		reply.Status = stwrpc.NoSuchTargetUser
		return nil
	}
	err = ts.storage.AppendToList(slistKey, args.TargetUserID)
	if err!=nil {
		reply.Status = stwrpc.Exists
	} else {
		reply.Status = stwrpc.OK
	}
	return nil
}

func (ts *stwServer) Unsubscribe(args *stwrpc.SubscriptionArgs, reply *stwrpc.SubscriptionReply) error {
	sKey := util.FormatUserKey(args.UserID)
	tKey := util.FormatUserKey(args.TargetUserID)
	slistKey := util.FormatSubListKey(args.UserID)
	_, err := ts.storage.Get(sKey)
	if err != nil {
		reply.Status = stwrpc.NoSuchUser
		return nil
	}
	_, err = ts.storage.Get(tKey)
	if err != nil {
		reply.Status = stwrpc.NoSuchTargetUser
		return nil
	}
	err = ts.storage.RemoveFromList(slistKey, args.TargetUserID)
	if err!=nil {
		reply.Status = stwrpc.NoSuchTargetUser
	} else {
		reply.Status = stwrpc.OK
	}
	return nil
}

func (ts *stwServer) Post(args *stwrpc.PostArgs, reply *stwrpc.PostReply) error {
	key := util.FormatUserKey(args.UserID)
	_, err := ts.storage.Get(key)
	if err != nil {
		reply.Status = stwrpc.NoSuchUser
		return nil
	}
	userPostListKey := util.FormatPostListKey(args.UserID)
	postkey := util.FormatPostKey(args.UserID, time.Now().UnixNano())
	ts.storage.Put(postkey, args.Contents)
	ts.storage.AppendToList(userPostListKey, postkey)
	reply.Status = stwrpc.OK
	reply.PostKey = postkey
	return nil
}

func (ts *stwServer) DeletePost(args *stwrpc.DeletePostArgs, reply *stwrpc.DeletePostReply) error {
	key := util.FormatUserKey(args.UserID)
	_, err := ts.storage.Get(key)
	if err != nil {
		reply.Status = stwrpc.NoSuchUser
		return nil
	}
	post_uid, _, err := util.ParsePostKey(args.PostKey)
	if err!=nil || post_uid != args.UserID {
		reply.Status = stwrpc.NoSuchPost
		return nil
	}
	userPostListKey := util.FormatPostListKey(args.UserID)
	err1 := ts.storage.RemoveFromList(userPostListKey, args.PostKey)
	if err1!=nil {
		reply.Status = stwrpc.NoSuchPost
		return nil
	}
	err2 := ts.storage.Delete(args.PostKey)
	if err2!=nil {
		reply.Status = stwrpc.NoSuchPost
		return nil
	}
	reply.Status = stwrpc.OK
	return nil
}

type ByRevChronological []string

func (a ByRevChronological) Len() int { return len(a) }
func (a ByRevChronological) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByRevChronological) Less(i, j int) bool {
	_, t1, _ := util.ParsePostKey(a[i])
	_, t2, _ := util.ParsePostKey(a[j])
	return t1 > t2
}

func (ts *stwServer) Timeline(args *stwrpc.TimelineArgs, reply *stwrpc.TimelineReply) error {
	key := util.FormatUserKey(args.UserID)
	_, err := ts.storage.Get(key)
	if err != nil {
		reply.Status = stwrpc.NoSuchUser
		return nil
	}
	userPostListKey := util.FormatPostListKey(args.UserID)
	postlist, err := ts.storage.GetList(userPostListKey)
	if err != nil {
		reply.Status = stwrpc.OK
		return nil
	}

	sort.Sort(ByRevChronological(postlist))

	limit := 100
	for _, pKey := range postlist {
		if limit <= 0 { break }
		post, err := ts.storage.Get(pKey)
		if err!=nil {
			log.Println("Can't find Post "+pKey+" of user "+args.UserID)
			return err
		}
		userID, unixTime, _ := util.ParsePostKey(pKey)

		// if unixTime > old_time {
		// 	log.Println("2 out of order:", old_key, pKey)
		// 	_, t1 := util.ParsePostKey(old_key)
		// 	_, t2 := util.ParsePostKey(pKey)
		// 	log.Println(t1, t2)
		// 	log.Println(t1 > t2)
		// }
		// old_time = unixTime
		// old_key = pKey

		reply.Posts = append(reply.Posts, stwrpc.Post{userID, strconv.FormatInt(unixTime, 16), post})
		limit--
	}
	reply.Status = stwrpc.OK
	return nil
}

func (ts *stwServer) HomeTimeline(args *stwrpc.TimelineArgs, reply *stwrpc.TimelineReply) error {
	key := util.FormatUserKey(args.UserID)
	_, err := ts.storage.Get(key)
	if err != nil {
		reply.Status = stwrpc.NoSuchUser
		return nil
	}
	subListKey := util.FormatSubListKey(args.UserID)
	slist, err := ts.storage.GetList(subListKey)
	if err!=nil {
		slist = []string{}
	}
	//slist = append(slist, args.UserID)
	pKeys := make([]string, 0)
	for _, t := range slist {
		tPostListKey := util.FormatPostListKey(t)
		postlist, err := ts.storage.GetList(tPostListKey)
		if err != nil {
			continue
		}
		limit := 100
		if len(postlist) < limit {
			limit = len(postlist)
		}
		sort.Sort(ByRevChronological(postlist))
		pKeys = append(pKeys, postlist[:limit]...)
	}
	sort.Sort(ByRevChronological(pKeys))
	limit := 100
	for _, pKey := range pKeys {
		if limit <= 0 { break }
		userID, unixTime, _ := util.ParsePostKey(pKey)
		post, err := ts.storage.Get(pKey)
		if err!=nil {
			log.Println("Can't find Post "+pKey+" of user "+userID)
			return err
		}
		reply.Posts = append(reply.Posts, stwrpc.Post{userID, strconv.FormatInt(unixTime, 16), post})
		limit--
	}
	reply.Status = stwrpc.OK
	return nil
}
