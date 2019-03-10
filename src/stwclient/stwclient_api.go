package stwclient

import "rpc/stwrpc"

type StwClient interface {
	CreateUser(userID string) (stwrpc.Status, error)
	Subscribe(userID, targetUser string) (stwrpc.Status, error)
	Unsubscribe(userID, targetUser string) (stwrpc.Status, error)
	Timeline(userID string) ([]stwrpc.Post, stwrpc.Status, error)
	HomeTimeline(userID string) ([]stwrpc.Post, stwrpc.Status, error)
	Post(userID, contents string) (stwrpc.PostReply, error)
	DeletePost(userID, postKey string) (stwrpc.Status, error)
	Close() error
}
