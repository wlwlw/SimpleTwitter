package webserver

import (
	"fmt"
	"errors"
	//"net"
 	"net/http"
 	"net/rpc"
 	"time"
 	"log"
 	"io/ioutil"
	"encoding/json"
	"strings"

 	"rpc/stwrpc"
 	"github.com/willf/bloom"
)

type webServer struct {
	stwServers []string
	stwConns map[string]*rpc.Client
	mux *http.ServeMux
	zombieFilter *bloom.BloomFilter
	underHighLoad bool
	avgLatency float64
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
	switch r.Method {
	case http.MethodGet:
	    echoHandler(w,r)
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

	    args2 := &stwrpc.CreateUserArgs{UserID: uid}
		var reply stwrpc.CreateUserReply
		err = cli.Call("StwServer.CreateUser", args2, &reply)
		if err!=nil {
			log.Println(err)
		}
		json.NewEncoder(w).Encode(reply)
	}
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
		json.NewEncoder(w).Encode(reply)
	case http.MethodDelete:
	    // Remove the record.
		err := cli.Call("StwServer.Unsubscribe", args, &reply)
		if err!=nil {
			log.Println(err)
		}
		json.NewEncoder(w).Encode(reply)
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
	    json.NewEncoder(w).Encode(reply)

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

func (ws *webServer) servePuzzle(w http.ResponseWriter, r *http.Request){
	remoteIP := strings.Split(r.RemoteAddr,":")[0]
	switch r.Method {
	case http.MethodPost:
		isZombie := ws.zombieFilter.Test([]byte(remoteIP))
		if isZombie {
			ws.zombieFilter.Remove([]byte(remoteIP))
		}
		//set passcode in cookie
		http.SetCookie(w, &http.Cookie{
			Name:    "passcode",
			Value:   "passcode",
			Expires: time.Now().Add(111 * time.Second),
		})
		ws.mux.ServeHTTP(w,r)
	case http.MethodGet:
		ws.zombieFilter.Add([]byte(remoteIP))
		fmt.Fprintf(w, 
			"<html><head>Confirm you are not Bot</head>"+
			"<body><form method='post'>"+
			"<input type='submit' value='ImNotBot'>"+
			"</form></body></html>",
		)
	}
}

func (ws *webServer) ddosProtectionWrapper(w http.ResponseWriter, r *http.Request){
	remoteIP := strings.Split(r.RemoteAddr,":")[0]

	//check cookie to see if answered puzzle
	_, err := r.Cookie("passcode")
	
	if err!=nil {
		isZombie := ws.zombieFilter.Test([]byte(remoteIP))
		if ws.underHighLoad || isZombie {
			// log.Println("Serve Puzzle")
			// enter Puzzle page
			ws.servePuzzle(w, r)
			return
		}
	}

	t1 := time.Now().UnixNano()
	ws.mux.ServeHTTP(w,r)
	delta := time.Now().UnixNano()-t1
	ws.avgLatency = 0.9*ws.avgLatency + 0.1*float64(delta)
	if ws.avgLatency/1000000>=20 {
		ws.underHighLoad = true
	}else{
		ws.underHighLoad = false
	}
}


func NewWebServer(myHostPort, masterStwServer string) (WebServer, error) {
	ws := &webServer{
		stwServers: []string{},
		stwConns: make(map[string]*rpc.Client),
		mux: http.NewServeMux(),
		zombieFilter: bloom.New(20000, 1),
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


	ws.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request){
        http.ServeFile(w, r, "./client/index.html")
    })
	ws.mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./client/"))))
	ws.mux.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("./client/"))))
	ws.mux.HandleFunc("/users", ws.usersHandler)
	ws.mux.HandleFunc("/subscriptions", ws.subscriptionHandler)
	ws.mux.HandleFunc("/posts", ws.postsHandler)
	ws.mux.HandleFunc("/timeline", ws.timelineHandler)
	ws.mux.HandleFunc("/home", ws.homeHandler)

	go http.ListenAndServe(myHostPort, http.HandlerFunc(ws.ddosProtectionWrapper))
	
	return ws, nil
}
