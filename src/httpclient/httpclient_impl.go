package httpclient

import (
	"net"
	//"net/rpc"
	"net/http"
	"strconv"
	"encoding/json"
	"bytes"
	"log"
	"os"

	"rpc/stwrpc"
)

var LOGE = log.New(os.Stderr, "", log.Lshortfile|log.Lmicroseconds)

type httpClient struct {
	serverAddr string
	client *http.Client
}

func NewHttpClient(serverHost string, serverPort int) (HttpClient, error) {
	tc := &httpClient{
		serverAddr: "http://"+net.JoinHostPort(serverHost, strconv.Itoa(serverPort)),
		client: &http.Client{},
	}
	return tc, nil
}

func (tc *httpClient) CreateUser(userID string) (stwrpc.Status, error) {
	args := &stwrpc.CreateUserArgs{UserID: userID}
	var reply stwrpc.CreateUserReply

	jsonStr, _ := json.Marshal(args)
	req, err := http.NewRequest("POST", tc.serverAddr+"/users", bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	resp, err := tc.client.Do(req)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()
    decoder := json.NewDecoder(resp.Body)

    err = decoder.Decode(&reply)
    if err!=nil {
		return 0, err
	}
	return reply.Status, nil
}

func (tc *httpClient) Subscribe(userID, targetUserID string) (stwrpc.Status, error) {
	return tc.doSub("POST", userID, targetUserID)
}

func (tc *httpClient) Unsubscribe(userID, targetUserID string) (stwrpc.Status, error) {
	return tc.doSub("DELETE", userID, targetUserID)
}

func (tc *httpClient) doSub(method, userID, targetUserID string) (stwrpc.Status, error) {
	args := &stwrpc.SubscriptionArgs{UserID: userID, TargetUserID: targetUserID}
	var reply stwrpc.SubscriptionReply

	req, err := http.NewRequest(method, tc.serverAddr+"/subscriptions", nil)
	req.Header.Set("Content-Type", "application/json")

	q := req.URL.Query()
	q.Add("UserID", args.UserID)
	q.Add("TargetUserID", args.TargetUserID)
	req.URL.RawQuery = q.Encode()

	resp, err := tc.client.Do(req)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()
    decoder := json.NewDecoder(resp.Body)
    
    err = decoder.Decode(&reply)
    if err!=nil {
		return 0, err
	}
	return reply.Status, nil
}

func (tc *httpClient) Timeline(userID string) ([]stwrpc.Post, stwrpc.Status, error) {
	args := &stwrpc.TimelineArgs{UserID: userID}
	var reply stwrpc.TimelineReply

	req, err := http.NewRequest("GET", tc.serverAddr+"/timeline", nil)
	req.Header.Set("Content-Type", "application/json")
	
	q := req.URL.Query()
	q.Add("UserID", args.UserID)
	req.URL.RawQuery = q.Encode()

	resp, err := tc.client.Do(req)
    if err != nil {
    	LOGE.Println(err)
        return nil, 0, err
    }
    defer resp.Body.Close()
    decoder := json.NewDecoder(resp.Body)
    
    err = decoder.Decode(&reply)
    if err!=nil {
		return nil, 0, err
	}
	return reply.Posts, reply.Status, nil
}

func (tc *httpClient) HomeTimeline(userID string) ([]stwrpc.Post, stwrpc.Status, error) {
	args := &stwrpc.TimelineArgs{UserID: userID}
	var reply stwrpc.TimelineReply

	req, err := http.NewRequest("GET", tc.serverAddr+"/home", nil)
	req.Header.Set("Content-Type", "application/json")

	q := req.URL.Query()
	q.Add("UserID", args.UserID)
	req.URL.RawQuery = q.Encode()

	resp, err := tc.client.Do(req)
    if err != nil {
        return nil, 0, err
    }
    defer resp.Body.Close()
    decoder := json.NewDecoder(resp.Body)
    
    err = decoder.Decode(&reply)
    if err != nil {
		return nil, 0, err
	}
	return reply.Posts, reply.Status, nil
}

func (tc *httpClient) Post(userID, contents string) (stwrpc.PostReply, error) {
	args := &stwrpc.PostArgs{UserID: userID, Contents: contents}
	var reply stwrpc.PostReply
	jsonStr, _ := json.Marshal(args)
	req, err := http.NewRequest("POST", tc.serverAddr+"/posts", bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	resp, err := tc.client.Do(req)
    if err != nil {
        return reply, err
    }
    defer resp.Body.Close()
    decoder := json.NewDecoder(resp.Body)
    
    err = decoder.Decode(&reply)
    if err!=nil {
		return reply, err
	}
	return reply, nil
}

func (tc *httpClient) DeletePost(userID, postKey string) (stwrpc.Status, error) {
	args := &stwrpc.DeletePostArgs{UserID: userID, PostKey: postKey}
	var reply stwrpc.DeletePostReply
	jsonStr, _ := json.Marshal(args)
	req, err := http.NewRequest("DELETE", tc.serverAddr+"/posts", bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	resp, err := tc.client.Do(req)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()
    decoder := json.NewDecoder(resp.Body)
    
    err = decoder.Decode(&reply)
    if err!=nil {
		return 0, err
	}
	return reply.Status, nil
}

func (tc *httpClient) DownloadIMG() error {
	req, err := http.NewRequest("GET", tc.serverAddr+"/assets/images/clock.png", nil)
	req.Header.Set("Content-Type", "image/png")

	resp, err := tc.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    buffer := make([]byte, resp.ContentLength)
    resp.Body.Read(buffer)
	return nil
}

func (tc *httpClient) Close() error {
	return nil
}
