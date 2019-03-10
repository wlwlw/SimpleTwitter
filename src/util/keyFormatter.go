package util

import (
	"fmt"
	//"math/rand"
	//"time"
	"strings"
	"strconv"
	"errors"
)

// format keys for User
// example: roc => roc:usrid
func FormatUserKey(userID string) string {
	return fmt.Sprintf("%s:usrid", userID)
}

// format key to associate with a user's subscription list
// example roc => roc:sublist
func FormatSubListKey(userID string) string {
	return fmt.Sprintf("%s:sublist", userID)
}

// format key for a post
// example roc make a post => roc:post_time_srvId (time and srvId in %x)
// srvId is a random number to break ties for post id, not perfect but will work with very high probability.
// If it turns out to be not unique, call this function again to generate a new one.
func FormatPostKey(userID string, postTime int64) string {
	return fmt.Sprintf("%s:post_%x", userID, postTime)
}

func ParsePostKey(postKey string) (userID string, postTime int64, e error) {
	slist := strings.Split(postKey, ":")
	if len(slist)<2 {
		return "", 0, errors.New("Invalid PostKey")
	}
	userID, res := slist[0], slist[1]
	slist = strings.Split(res, "_")
	if len(slist)<2 {
		return userID, 0, errors.New("Invalid PostKey")
	}
	postTime, _ = strconv.ParseInt(slist[1], 16, 64)
	return userID, postTime, nil
}


// format key to associate with a user's list for post keys
func FormatPostListKey(userID string) string {
	return fmt.Sprintf("%s:postlist", userID)
}
