package stwrpc

type RemoteStwServer interface {
	RegisterServer(args*RegisterArgs, reply *RegisterReply) error
	GetServers(args *GetServersArgs, reply *GetServersReply) error
	CreateUser(args *CreateUserArgs, reply *CreateUserReply) error
	Subscribe(args *SubscriptionArgs, reply *SubscriptionReply) error
	Unsubscribe(args *SubscriptionArgs, reply *SubscriptionReply) error
	Post(args *PostArgs, reply *PostReply) error
	DeletePost(args *DeletePostArgs, reply *DeletePostReply) error
	Timeline(args *TimelineArgs, reply *TimelineReply) error
	HomeTimeline(args *TimelineArgs, reply *TimelineReply) error
}

type StwServer struct {
	RemoteStwServer
}

func Wrap(t RemoteStwServer) RemoteStwServer {
	return &StwServer{t}
}
