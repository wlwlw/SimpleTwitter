package stwrpc

//import "time"

type Status int

const (
	OK               Status = iota + 1 // The RPC was a success.
	NoSuchUser                         // The specified UserID does not exist.
	NoSuchPost                         // The specified PostKey does not exist.
	NoSuchTargetUser                   // The specified TargerUserID does not exist.
	Exists                             // The specified UserID or TargerUserID already exists.
	NotReady                           // The app servers are still getting ready.
)

type Node struct {
	HostPort string // The host:port address of the storage server node.
}

type RegisterArgs struct {
	ServerInfo Node
}

type RegisterReply struct {
	Status  Status
	Servers []Node
}

type GetServersArgs struct {
	// Intentionally left empty.
}

type GetServersReply struct {
	Status  Status
	Servers []Node
}


type Post struct {
	UserID   string    
	Posted   string
	Contents string
}

type CreateUserArgs struct {
	UserID string
}

type CreateUserReply struct {
	Status Status
}

type SubscriptionArgs struct {
	UserID       string
	TargetUserID string
}

type SubscriptionReply struct {
	Status Status
}

type PostArgs struct {
	UserID   string
	Contents string
}

type PostReply struct {
	PostKey string
	Status  Status
}

type DeletePostArgs struct {
	UserID  string
	PostKey string
}

type DeletePostReply struct {
	Status Status
}

type TimelineArgs struct {
	UserID string
}

type TimelineReply struct {
	Status   Status
	Posts []Post
}
