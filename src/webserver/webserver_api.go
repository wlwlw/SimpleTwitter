package webserver

import (
	"hash/fnv"
)

type WebServer interface {
}

// RequestHash hashes a userID and returns a 32-bit integer. This function
// defines the request routing between webserver and appserver (so webserver 
// can act as load balancer)
func RequestHash(userID string) uint32 {
	hasher := fnv.New32()
	hasher.Write([]byte(userID))
	return hasher.Sum32()
}