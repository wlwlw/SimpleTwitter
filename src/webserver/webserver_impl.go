package webserver

import (
	//"fmt"
	"errors"
	//"net"
 	"net/http"
 	"net/rpc"
 	"time"
 	"log"
 	"io/ioutil"
	"encoding/json"

 	"rpc/stwrpc"
)

type webServer struct {
	stwServers []string
	stwConns map[string]*rpc.Client
}

func echoHandler(w http.ResponseWriter, r *http.Request){
	body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        log.Printf("Error reading body: %v", err)
        http.Error(w, "can't read body", http.StatusBadRequest)
        return
    }
    w.Write(body)
}

func (ws *webServer) getStwConn(key string) *rpc.Client {
	host := ws.stwServers[RequestHash(key)%uint32(len(ws.stwServers))]
	cli, ok := ws.stwConns[host]
	if !ok {
		cli, err := rpc.DialHTTP("tcp", host)
		if err != nil {
			log.Fatal("rpc error:", err)
		}
		ws.stwConns[host] = cli
		return cli
	}
	return cli
}

func (ws *webServer) usersHandler(w http.ResponseWriter, r *http.Request){
    echoHandler(w,r)
}
func (ws *webServer) subscriptionHandler(w http.ResponseWriter, r *http.Request){
	ss, ok := r.URL.Query()["UserID"]
    if !ok {
    	w.WriteHeader(http.StatusBadRequest)
		return
    }
    ts, ok := r.URL.Query()["TargetUserID"]
    if !ok {
    	w.WriteHeader(http.StatusBadRequest)
		return
    }
    s, t := ss[0], ts[0]
    cli := ws.getStwConn(s)

    args := &stwrpc.SubscriptionArgs{UserID: s, TargetUserID: t}
	var reply stwrpc.SubscriptionReply

	switch r.Method {
	case http.MethodGet:
	    echoHandler(w,r)
	case http.MethodPost:
	    // Create a new record.
		err := cli.Call("StwServer.Subscribe", args, &reply)
		if err!=nil {
			log.Println(err)
		}

	case http.MethodDelete:
	    // Remove the record.
		err := cli.Call("StwServer.Unsubscribe", args, &reply)
		if err!=nil {
			log.Println(err)
		}

	default:
	    w.WriteHeader(http.StatusBadRequest)
	    echoHandler(w,r)
	}

}
func (ws *webServer) postsHandler(w http.ResponseWriter, r *http.Request){
	switch r.Method {
	case http.MethodPost:
		decoder := json.NewDecoder(r.Body)
	    var args stwrpc.PostArgs
	    err := decoder.Decode(&args)
	    if err!=nil {
	    	w.WriteHeader(http.StatusBadRequest)
	    	echoHandler(w,r)
			return
	    }

	    uid := args.UserID
	    cli := ws.getStwConn(uid)

	    //create user without authorization for now
	    args0 := &stwrpc.CreateUserArgs{UserID: uid}
		var reply0 stwrpc.CreateUserReply
		err = cli.Call("StwServer.CreateUser", args0, &reply0)
		if err!=nil {
			log.Println(err)
		}

		var reply stwrpc.PostReply
		if err = cli.Call("StwServer.Post", &args, &reply); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(reply)
	case http.MethodDelete:
		uids, ok := r.URL.Query()["UserID"]
	    if !ok {
	    	w.WriteHeader(http.StatusBadRequest)
			return
	    }
	    postKeys, ok := r.URL.Query()["PostKey"]
	    if !ok {
	    	w.WriteHeader(http.StatusBadRequest)
			return
	    }
	    uid, postKey := uids[0], postKeys[0]
	    cli := ws.getStwConn(uid)

	    args := &stwrpc.DeletePostArgs{UserID:uid, PostKey:postKey}
	    var reply stwrpc.DeletePostReply
	    if err := cli.Call("StwServer.DeletePost", args, &reply); err!=nil {
	    	log.Println(err)
	    	w.WriteHeader(http.StatusBadRequest)
			return
	    }

	default:
		w.WriteHeader(http.StatusBadRequest)
	    echoHandler(w,r)
	}
}
func (ws *webServer) timelineHandler(w http.ResponseWriter, r *http.Request){
    uids, ok := r.URL.Query()["UserID"]
    if !ok {
    	w.WriteHeader(http.StatusBadRequest)
    	echoHandler(w,r)
		return
    }

    args := stwrpc.TimelineArgs{UserID: uids[0]}

    uid := args.UserID
    cli := ws.getStwConn(uid)

    //create user without authorization for now
    args0 := &stwrpc.CreateUserArgs{UserID: uid}
	var reply0 stwrpc.CreateUserReply
	err := cli.Call("StwServer.CreateUser", args0, &reply0)
	if err!=nil {
		log.Println(err)
	}

	var reply stwrpc.TimelineReply
	if err = cli.Call("StwServer.Timeline", &args, &reply); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(reply)
}

func (ws *webServer) homeHandler(w http.ResponseWriter, r *http.Request){
    uids, ok := r.URL.Query()["UserID"]
    if !ok {
    	w.WriteHeader(http.StatusBadRequest)
    	echoHandler(w,r)
		return
    }

    args := stwrpc.TimelineArgs{UserID: uids[0]}

    uid := args.UserID
    cli := ws.getStwConn(uid)

    //create user without authorization for now
    args0 := &stwrpc.CreateUserArgs{UserID: uid}
	var reply0 stwrpc.CreateUserReply
	err := cli.Call("StwServer.CreateUser", args0, &reply0)
	if err!=nil {
		log.Println(err)
	}

	//make user subscribe themselves
	args1 := &stwrpc.SubscriptionArgs{UserID: uid, TargetUserID: uid}
	var reply1 stwrpc.SubscriptionReply
	err = cli.Call("StwServer.Subscribe", args1, &reply1)
	if err!=nil {
		log.Println(err)
	}

	var reply stwrpc.TimelineReply
	if err = cli.Call("StwServer.HomeTimeline", &args, &reply); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(reply)
}



func NewWebServer(myHostPort, masterStwServer string) (WebServer, error) {
	ws := &webServer{
		stwServers: []string{},
		stwConns: make(map[string]*rpc.Client),
	}

	if masterStwServer != "" {
		client, err := rpc.DialHTTP("tcp", masterStwServer)
		if err != nil {
			return nil, err
		}
		for re := 0; ; re++ {
			if re >= 5 {
				return nil, errors.New("Timeout when waiting Appserver ready.")
			}
			args := stwrpc.GetServersArgs{}
			reply := stwrpc.GetServersReply{}
			err = client.Call("StwServer.GetServers", args, &reply)
			if err != nil {
				log.Fatal("rpc error:", err)
			}
			if reply.Status == stwrpc.OK {
				for _, node := range reply.Servers {
					addr := node.HostPort
					ws.stwServers = append(ws.stwServers, addr)
				}
				break
			} else if reply.Status == stwrpc.NotReady {
				time.Sleep(1 * time.Second)
			}
		}
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request){
        http.ServeFile(w, r, "./client/index.html")
    })
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./client/"))))
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("./client/"))))
	http.HandleFunc("/users", ws.usersHandler)
	http.HandleFunc("/subscriptions", ws.subscriptionHandler)
	http.HandleFunc("/posts", ws.postsHandler)
	http.HandleFunc("/timeline", ws.timelineHandler)
	http.HandleFunc("/home", ws.homeHandler)

	s := &http.Server{
		Addr:           myHostPort,
		Handler:        nil,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	err := s.ListenAndServe()
	if err != nil {
        log.Fatal("ListenAndServe: ", err)
    }
	
	return ws, nil
}
