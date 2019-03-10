package stwserver

import "rpc/stwrpc"

type StwServer interface {

	// RegisterServer adds a server to the cluster. It replies with
	// status NotReady if not all nodes in the cluster have joined. Once
	// all nodes have joined, it should reply with status OK and a list
	// of all connected nodes in the cluster.
	RegisterServer(args *stwrpc.RegisterArgs, reply *stwrpc.RegisterReply) error

	// GetServers retrieves a list of all connected nodes in the cluster. It
	// replies with status NotReady if not all nodes in the cluster have joined.
	GetServers(args *stwrpc.GetServersArgs, reply *stwrpc.GetServersReply) error


	CreateUser(args *stwrpc.CreateUserArgs, reply *stwrpc.CreateUserReply) error

	Subscribe(args *stwrpc.SubscriptionArgs, reply *stwrpc.SubscriptionReply) error

	Unsubscribe(args *stwrpc.SubscriptionArgs, reply *stwrpc.SubscriptionReply) error

	Post(args *stwrpc.PostArgs, reply *stwrpc.PostReply) error

	DeletePost(args *stwrpc.DeletePostArgs, reply *stwrpc.DeletePostReply) error

	Timeline(args *stwrpc.TimelineArgs, reply *stwrpc.TimelineReply) error

	HomeTimeline(args *stwrpc.TimelineArgs, reply *stwrpc.TimelineReply) error
}
