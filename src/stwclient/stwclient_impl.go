package stwclient

import (
	"net"
	"net/rpc"
	"strconv"

	"rpc/stwrpc"
)

type stwClient struct {
	client *rpc.Client
}

func NewStwClient(serverHost string, serverPort int) (StwClient, error) {
	cli, err := rpc.DialHTTP("tcp", net.JoinHostPort(serverHost, strconv.Itoa(serverPort)))
	if err != nil {
		return nil, err
	}
	return &stwClient{client: cli}, nil
}

func (tc *stwClient) CreateUser(userID string) (stwrpc.Status, error) {
	args := &stwrpc.CreateUserArgs{UserID: userID}
	var reply stwrpc.CreateUserReply
	if err := tc.client.Call("StwServer.CreateUser", args, &reply); err != nil {
		return 0, err
	}
	return reply.Status, nil
}

func (tc *stwClient) Subscribe(userID, targetUserID string) (stwrpc.Status, error) {
	return tc.doSub("StwServer.Subscribe", userID, targetUserID)
}

func (tc *stwClient) Unsubscribe(userID, targetUserID string) (stwrpc.Status, error) {
	return tc.doSub("StwServer.Unsubscribe", userID, targetUserID)
}

func (tc *stwClient) doSub(funcName, userID, targetUserID string) (stwrpc.Status, error) {
	args := &stwrpc.SubscriptionArgs{UserID: userID, TargetUserID: targetUserID}
	var reply stwrpc.SubscriptionReply
	if err := tc.client.Call(funcName, args, &reply); err != nil {
		return 0, err
	}
	return reply.Status, nil
}

func (tc *stwClient) Timeline(userID string) ([]stwrpc.Post, stwrpc.Status, error) {
	args := &stwrpc.TimelineArgs{UserID: userID}
	var reply stwrpc.TimelineReply
	if err := tc.client.Call("StwServer.Timeline", args, &reply); err != nil {
		return nil, 0, err
	}
	return reply.Posts, reply.Status, nil
}

func (tc *stwClient) HomeTimeline(userID string) ([]stwrpc.Post, stwrpc.Status, error) {
	args := &stwrpc.TimelineArgs{UserID: userID}
	var reply stwrpc.TimelineReply
	if err := tc.client.Call("StwServer.HomeTimeline", args, &reply); err != nil {
		return nil, 0, err
	}
	return reply.Posts, reply.Status, nil
}

func (tc *stwClient) Post(userID, contents string) (stwrpc.PostReply, error) {
	args := &stwrpc.PostArgs{UserID: userID, Contents: contents}
	var reply stwrpc.PostReply
	if err := tc.client.Call("StwServer.Post", args, &reply); err != nil {
		return reply, err
	}
	return reply, nil
}

func (tc *stwClient) DeletePost(userID, postKey string) (stwrpc.Status, error) {
	args := &stwrpc.DeletePostArgs{UserID: userID, PostKey: postKey}
	var reply stwrpc.DeletePostReply
	if err := tc.client.Call("StwServer.DeletePost", args, &reply); err != nil {
		return 0, err
	}
	return reply.Status, nil
}

func (tc *stwClient) Close() error {
	return tc.client.Close()
}
