package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"regexp"
	"strconv"
	"strings"

	"rpc/storagerpc"
	"rpc/stwrpc"
	"tests/proxycounter"
	"stwserver"
)

type testFunc struct {
	name string
	f    func()
}

var (
	port      = flag.Int("port", 9010, "StwServer port number")
	testRegex = flag.String("t", "", "test to run")
	passCount int
	failCount int
	pc        proxycounter.ProxyCounter
	ts        stwserver.StwServer
)

var statusMap = map[stwrpc.Status]string{
	stwrpc.OK:               "OK",
	stwrpc.NoSuchUser:       "NoSuchUser",
	stwrpc.NoSuchPost:       "NoSuchPost",
	stwrpc.NoSuchTargetUser: "NoSuchTargetUser",
	stwrpc.Exists:           "Exists",
	0:                        "Unknown",
}

var LOGE = log.New(os.Stderr, "", log.Lshortfile|log.Lmicroseconds)

func initStwServer(masterServerHostPort string, stwServerPort int) error {
	stwServerHostPort := net.JoinHostPort("localhost", strconv.Itoa(stwServerPort))
	proxyCounter, err := proxycounter.NewProxyCounter(masterServerHostPort, stwServerHostPort)
	if err != nil {
		LOGE.Println("Failed to setup test:", err)
		return err
	}
	pc = proxyCounter
	rpc.RegisterName("StorageServer", storagerpc.Wrap(pc))

	// Create and start the StwServer.
	stwServer, err := stwserver.NewStwServer(stwServerHostPort, "", masterServerHostPort, 1)
	if err != nil {
		LOGE.Println("Failed to create StwServer:", err)
		return err
	}
	ts = stwServer
	return nil
}

// Cleanup stwserver and rpc hooks
func cleanupStwServer(l net.Listener) {
	// Close listener to stop http serve thread
	if l != nil {
		l.Close()
	}
	// Recreate default http serve mux
	http.DefaultServeMux = http.NewServeMux()
	// Recreate default rpc server
	rpc.DefaultServer = rpc.NewServer()
	// Unset stwserver just in case
	ts = nil
}

// Check rpc and byte count limits.
func checkLimits(rpcCountLimit, byteCountLimit uint32) bool {
	if pc.GetRpcCount() > rpcCountLimit {
		LOGE.Println("FAIL: using too many RPCs")
		failCount++
		return true
	}
	if pc.GetByteCount() > byteCountLimit {
		LOGE.Println("FAIL: transferring too much data")
		failCount++
		return true
	}
	return false
}

// Check error and status
func checkErrorStatus(err error, status, expectedStatus stwrpc.Status) bool {
	if err != nil {
		LOGE.Println("FAIL: unexpected error returned:", err)
		failCount++
		return true
	}
	if status != expectedStatus {
		LOGE.Printf("FAIL: incorrect status %s, expected status %s\n", statusMap[status], statusMap[expectedStatus])
		failCount++
		return true
	}
	return false
}

// Check subscriptions
func checkSubscriptions(subs, expectedSubs []string) bool {
	if len(subs) != len(expectedSubs) {
		LOGE.Printf("FAIL: incorrect subscriptions %v, expected subscriptions %v\n", subs, expectedSubs)
		failCount++
		return true
	}
	m := make(map[string]bool)
	for _, s := range subs {
		m[s] = true
	}
	for _, s := range expectedSubs {
		if m[s] == false {
			LOGE.Printf("FAIL: incorrect subscriptions %v, expected subscriptions %v\n", subs, expectedSubs)
			failCount++
			return true
		}
	}
	return false
}

// Check friends
func checkFriends(friends, expectedFriends []string) bool {
	if len(friends) != len(expectedFriends) {
		LOGE.Printf("FAIL: incorrect friends %v, expected friends %v\n", friends, expectedFriends)
		failCount++
		return true
	}
	m := make(map[string]bool)
	for _, f := range friends {
		m[f] = true
	}
	for _, f := range expectedFriends {
		if m[f] == false {
			LOGE.Printf("FAIL: incorrect friends %v, expected friends %v\n", friends, expectedFriends)
			failCount++
			return true
		}
	}
	return false
}

// Check posts
func checkPosts(posts, expectedPosts []stwrpc.Post) bool {
	if len(posts) != len(expectedPosts) {
		LOGE.Printf("FAIL: incorrect posts %v, expected posts %v\n", posts, expectedPosts)
		failCount++
		return true
	}
	lastTime := int64(0)
	for i := len(posts) - 1; i >= 0; i-- {
		if posts[i].UserID != expectedPosts[i].UserID {
			LOGE.Printf("FAIL: incorrect posts %v, expected posts %v\n", posts, expectedPosts)
			failCount++
			return true
		}
		if posts[i].Contents != expectedPosts[i].Contents {
			LOGE.Printf("FAIL: incorrect posts %v, expected posts %v\n", posts, expectedPosts)
			failCount++
			return true
		}
		curTime, _ := strconv.ParseInt(posts[i].Posted, 16, 64)
		if curTime < lastTime {
			LOGE.Println("FAIL: post timestamps not in reverse chronological order")
			failCount++
			return true
		}
		lastTime = curTime
	}
	return false
}

// Helper functions
func createUser(user string) (error, stwrpc.Status) {
	args := &stwrpc.CreateUserArgs{UserID: user}
	var reply stwrpc.CreateUserReply
	err := ts.CreateUser(args, &reply)
	return err, reply.Status
}

func addSubscription(user, target string) (error, stwrpc.Status) {
	args := &stwrpc.SubscriptionArgs{UserID: user, TargetUserID: target}
	var reply stwrpc.SubscriptionReply
	err := ts.Subscribe(args, &reply)
	return err, reply.Status
}

func removeSubscription(user, target string) (error, stwrpc.Status) {
	args := &stwrpc.SubscriptionArgs{UserID: user, TargetUserID: target}
	var reply stwrpc.SubscriptionReply
	err := ts.Unsubscribe(args, &reply)
	return err, reply.Status
}

func post(user, contents string) (error, stwrpc.Status) {
	args := &stwrpc.PostArgs{UserID: user, Contents: contents}
	var reply stwrpc.PostReply
	err := ts.Post(args, &reply)
	return err, reply.Status
}

func post2(user, contents string) (error, stwrpc.Status, string) {
	args := &stwrpc.PostArgs{UserID: user, Contents: contents}
	var reply stwrpc.PostReply
	err := ts.Post(args, &reply)
	return err, reply.Status, reply.PostKey
}

func deletePost(user, postKey string) (error, stwrpc.Status) {
	args := &stwrpc.DeletePostArgs{UserID: user, PostKey: postKey}
	var reply stwrpc.DeletePostReply
	err := ts.DeletePost(args, &reply)
	return err, reply.Status
}

func getPosts(user string) (error, stwrpc.Status, []stwrpc.Post) {
	args := &stwrpc.TimelineArgs{UserID: user}
	var reply stwrpc.TimelineReply
	err := ts.Timeline(args, &reply)
	return err, reply.Status, reply.Posts
}

func getPostsBySubscription(user string) (error, stwrpc.Status, []stwrpc.Post) {
	args := &stwrpc.TimelineArgs{UserID: user}
	var reply stwrpc.TimelineReply
	err := ts.HomeTimeline(args, &reply)
	return err, reply.Status, reply.Posts
}

// Create valid user
func testCreateUserValid() {
	pc.Reset()
	err, status := createUser("user")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Create duplicate user
func testCreateUserDuplicate() {
	createUser("user")
	pc.Reset()
	err, status := createUser("user")
	if checkErrorStatus(err, status, stwrpc.Exists) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Add subscription with invalid user
func testSubscribeInvalidUser() {
	createUser("user")
	pc.Reset()
	err, status := addSubscription("invalidUser", "user")
	if checkErrorStatus(err, status, stwrpc.NoSuchUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Add subscription with invaild target user
func testSubscribeInvalidTargetUser() {
	createUser("user")
	pc.Reset()
	err, status := addSubscription("user", "invalidUser")
	if checkErrorStatus(err, status, stwrpc.NoSuchTargetUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Add valid subscription
func testSubscribeValid() {
	createUser("user1")
	createUser("user2")
	pc.Reset()
	err, status := addSubscription("user1", "user2")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Add duplicate subscription
func testSubscribeDuplicate() {
	createUser("user1")
	createUser("user2")
	addSubscription("user1", "user2")
	pc.Reset()
	err, status := addSubscription("user1", "user2")
	if checkErrorStatus(err, status, stwrpc.Exists) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Remove subscription with invalid user
func testUnsubscribeInvalidUser() {
	createUser("user")
	pc.Reset()
	err, status := removeSubscription("invalidUser", "user")
	if checkErrorStatus(err, status, stwrpc.NoSuchUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Remove valid subscription
func testUnsubscribeValid() {
	createUser("user1")
	createUser("user2")
	addSubscription("user1", "user2")
	pc.Reset()
	err, status := removeSubscription("user1", "user2")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Remove subscription with missing target user
func testUnsubscribeMissingTarget() {
	createUser("user1")
	createUser("user2")
	removeSubscription("user1", "user2")
	pc.Reset()
	err, status := removeSubscription("user1", "user2")
	if checkErrorStatus(err, status, stwrpc.NoSuchTargetUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Post post with invalid user
func testPostInvalidUser() {
	pc.Reset()
	err, status := post("invalidUser", "contents")
	if checkErrorStatus(err, status, stwrpc.NoSuchUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Post valid post
func testPostValid() {
	createUser("user")
	pc.Reset()
	err, status := post("user", "contents")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Delete post with invalid user
func testDeletePostInvalidUser() {
	pc.Reset()
	err, status := deletePost("invalidUser", "validPost")
	if checkErrorStatus(err, status, stwrpc.NoSuchUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

func testDeletePostInvalidPostKey() {
	createUser("user")
	pc.Reset()
	err, status, _ := post2("user", "contents")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	err, status = deletePost("user", "invalidPostKey")
	if checkErrorStatus(err, status, stwrpc.NoSuchPost) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Delete valid post
func testDeletePostValid() {
	createUser("user")
	pc.Reset()
	err, status, postKey := post2("user", "contents")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	err, status = deletePost("user", postKey)
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

func testDeletePostValid2() {
	createUser("stwUser200")
	expectedPosts := []stwrpc.Post{}
	numPosts := 5
	postKeys := make([]string, numPosts, numPosts)
	for i := 0; i < 5; i++ {
		expectedPosts = append(expectedPosts, stwrpc.Post{UserID: "stwUser200", Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedPosts) - 1; i >= 0; i-- {
		_, _, postKeys[i] = post2(expectedPosts[i].UserID, expectedPosts[i].Contents)
	}
	pc.Reset()

	// delete one post
	err, status := deletePost("stwUser200", postKeys[numPosts-1])
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}

	err, status, posts := getPosts("stwUser200")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkPosts(posts, expectedPosts[:numPosts-1]) {
		return
	}
	if checkLimits(50, 5000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get posts invalid user
func testTimelineInvalidUser() {
	pc.Reset()
	err, status, _ := getPosts("invalidUser")
	if checkErrorStatus(err, status, stwrpc.NoSuchUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get posts 0 posts
func testTimelineZeroPosts() {
	createUser("stwUser")
	pc.Reset()
	err, status, posts := getPosts("stwUser")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkPosts(posts, []stwrpc.Post{}) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get posts < 100 posts
func testTimelineFewPosts() {
	createUser("stwUser")
	expectedPosts := []stwrpc.Post{}
	for i := 0; i < 5; i++ {
		expectedPosts = append(expectedPosts, stwrpc.Post{UserID: "stwUser", Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedPosts) - 1; i >= 0; i-- {
		post(expectedPosts[i].UserID, expectedPosts[i].Contents)
	}
	pc.Reset()
	err, status, posts := getPosts("stwUser")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkPosts(posts, expectedPosts) {
		return
	}
	if checkLimits(50, 5000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get posts > 100 posts
func testTimelineManyPosts() {
	createUser("stwUser")
	post("stwUser", "should not see this old msg")
	expectedPosts := []stwrpc.Post{}
	for i := 0; i < 100; i++ {
		expectedPosts = append(expectedPosts, stwrpc.Post{UserID: "stwUser", Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedPosts) - 1; i >= 0; i-- {
		post(expectedPosts[i].UserID, expectedPosts[i].Contents)
	}
	pc.Reset()
	err, status, posts := getPosts("stwUser")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkPosts(posts, expectedPosts) {
		return
	}
	if checkLimits(200, 30000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get posts by subscription invalid user
func testHomeTimelineInvalidUser() {
	pc.Reset()
	err, status, _ := getPostsBySubscription("invalidUser")
	if checkErrorStatus(err, status, stwrpc.NoSuchUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get posts by subscription no subscriptions
func testHomeTimelineNoSubscriptions() {
	createUser("stwUser")
	post("stwUser", "contents")
	pc.Reset()
	err, status, posts := getPostsBySubscription("stwUser")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkPosts(posts, []stwrpc.Post{}) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get posts by subscription 0 posts
func testHomeTimelineZeroPosts() {
	createUser("stwUser1")
	createUser("stwUser2")
	addSubscription("stwUser1", "stwUser2")
	pc.Reset()
	err, status, posts := getPosts("stwUser1")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkPosts(posts, []stwrpc.Post{}) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get posts by subscription < 100 posts
func testHomeTimelineFewPosts() {
	createUser("stwUser1")
	createUser("stwUser2")
	createUser("stwUser3")
	createUser("stwUser4")
	addSubscription("stwUser1", "stwUser2")
	addSubscription("stwUser1", "stwUser3")
	addSubscription("stwUser1", "stwUser4")
	post("stwUser1", "should not see this unSubscribed msg")
	expectedPosts := []stwrpc.Post{stwrpc.Post{UserID: "stwUser2", Contents: "contents"}, stwrpc.Post{UserID: "stwUser4", Contents: "contents"}}
	for i := len(expectedPosts) - 1; i >= 0; i-- {
		post(expectedPosts[i].UserID, expectedPosts[i].Contents)
	}
	pc.Reset()
	err, status, posts := getPostsBySubscription("stwUser1")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkPosts(posts, expectedPosts) {
		return
	}
	if checkLimits(20, 2000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get posts by subscription > 100 posts
func testHomeTimelineManyPosts() {
	createUser("stwUser1")
	createUser("stwUser2")
	createUser("stwUser3")
	createUser("stwUser4")
	addSubscription("stwUser1", "stwUser2")
	addSubscription("stwUser1", "stwUser3")
	addSubscription("stwUser1", "stwUser4")
	post("stwUser1", "should not see this old msg")
	post("stwUser2", "should not see this old msg")
	post("stwUser3", "should not see this old msg")
	post("stwUser4", "should not see this old msg")
	expectedPosts := []stwrpc.Post{}
	for i := 0; i < 100; i++ {
		expectedPosts = append(expectedPosts, stwrpc.Post{UserID: fmt.Sprintf("stwUser%d", (i%3)+2), Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedPosts) - 1; i >= 0; i-- {
		post(expectedPosts[i].UserID, expectedPosts[i].Contents)
	}
	pc.Reset()
	err, status, posts := getPostsBySubscription("stwUser1")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkPosts(posts, expectedPosts) {
		return
	}
	if checkLimits(200, 30000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get posts by subscription all recent posts by one subscription
func testHomeTimelineManyPosts2() {
	createUser("stwUser1b")
	createUser("stwUser2b")
	createUser("stwUser3b")
	createUser("stwUser4b")
	addSubscription("stwUser1b", "stwUser2b")
	addSubscription("stwUser1b", "stwUser3b")
	addSubscription("stwUser1b", "stwUser4b")
	post("stwUser1b", "should not see this old msg")
	post("stwUser2b", "should not see this old msg")
	post("stwUser3b", "should not see this old msg")
	post("stwUser4b", "should not see this old msg")
	expectedPosts := []stwrpc.Post{}
	for i := 0; i < 100; i++ {
		expectedPosts = append(expectedPosts, stwrpc.Post{UserID: fmt.Sprintf("stwUser3b"), Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedPosts) - 1; i >= 0; i-- {
		post(expectedPosts[i].UserID, expectedPosts[i].Contents)
	}
	pc.Reset()
	err, status, posts := getPostsBySubscription("stwUser1b")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkPosts(posts, expectedPosts) {
		return
	}
	if checkLimits(200, 30000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get posts by subscription test not performing too many RPCs or transferring too much data
func testHomeTimelineManyPosts3() {
	createUser("stwUser1c")
	createUser("stwUser2c")
	createUser("stwUser3c")
	createUser("stwUser4c")
	createUser("stwUser5c")
	createUser("stwUser6c")
	createUser("stwUser7c")
	createUser("stwUser8c")
	createUser("stwUser9c")
	addSubscription("stwUser1c", "stwUser2c")
	addSubscription("stwUser1c", "stwUser3c")
	addSubscription("stwUser1c", "stwUser4c")
	addSubscription("stwUser1c", "stwUser5c")
	addSubscription("stwUser1c", "stwUser6c")
	addSubscription("stwUser1c", "stwUser7c")
	addSubscription("stwUser1c", "stwUser8c")
	addSubscription("stwUser1c", "stwUser9c")
	post("stwUser1c", "should not see this old msg")
	post("stwUser2c", "should not see this old msg")
	post("stwUser3c", "should not see this old msg")
	post("stwUser4c", "should not see this old msg")
	post("stwUser5c", "should not see this old msg")
	post("stwUser6c", "should not see this old msg")
	post("stwUser7c", "should not see this old msg")
	post("stwUser8c", "should not see this old msg")
	post("stwUser9c", "should not see this old msg")
	longContents := strings.Repeat("this sentence is 30 char long\n", 30)
	for i := 0; i < 100; i++ {
		for j := 1; j <= 9; j++ {
			post(fmt.Sprintf("stwUser%dc", j), longContents)
		}
	}
	expectedPosts := []stwrpc.Post{}
	for i := 0; i < 100; i++ {
		expectedPosts = append(expectedPosts, stwrpc.Post{UserID: fmt.Sprintf("stwUser%dc", (i%8)+2), Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedPosts) - 1; i >= 0; i-- {
		post(expectedPosts[i].UserID, expectedPosts[i].Contents)
	}
	pc.Reset()
	err, status, posts := getPostsBySubscription("stwUser1c")
	if checkErrorStatus(err, status, stwrpc.OK) {
		return
	}
	if checkPosts(posts, expectedPosts) {
		return
	}
	if checkLimits(200, 200000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

func main() {
	tests := []testFunc{
		{"testCreateUserValid", testCreateUserValid},
		{"testCreateUserDuplicate", testCreateUserDuplicate},
		{"testSubscribeInvalidUser", testSubscribeInvalidUser},
		{"testSubscribeInvalidTargetUser", testSubscribeInvalidTargetUser},
		{"testSubscribeValid", testSubscribeValid},
		{"testSubscribeDuplicate", testSubscribeDuplicate},
		{"testUnsubscribeInvalidUser", testUnsubscribeInvalidUser},
		{"testUnsubscribeValid", testUnsubscribeValid},
		{"testUnsubscribeMissingTarget", testUnsubscribeMissingTarget},
		{"testPostInvalidUser", testPostInvalidUser},
		{"testPostValid", testPostValid},
		{"testTimelineInvalidUser", testTimelineInvalidUser},
		{"testTimelineZeroPosts", testTimelineZeroPosts},
		{"testTimelineFewPosts", testTimelineFewPosts},
		{"testTimelineManyPosts", testTimelineManyPosts},
		{"testHomeTimelineInvalidUser", testHomeTimelineInvalidUser},
		{"testHomeTimelineNoSubscriptions", testHomeTimelineNoSubscriptions},
		{"testHomeTimelineZeroPosts", testHomeTimelineZeroPosts},
		{"testHomeTimelineZeroPosts", testHomeTimelineZeroPosts},
		{"testHomeTimelineFewPosts", testHomeTimelineFewPosts},
		{"testHomeTimelineManyPosts", testHomeTimelineManyPosts},
		{"testHomeTimelineManyPosts2", testHomeTimelineManyPosts2},
		{"testHomeTimelineManyPosts3", testHomeTimelineManyPosts3},
		{"testDeletePostInvalidUser", testDeletePostInvalidUser},
		{"testDeletePostInvalidPostKey", testDeletePostInvalidPostKey},
		{"testDeletePostValid", testDeletePostValid},
		{"testDeletePostValid2", testDeletePostValid2},
	}

	flag.Parse()
	if flag.NArg() < 1 {
		LOGE.Fatal("Usage: stwtest <storage master host:port>")
	}

	if err := initStwServer(flag.Arg(0), *port); err != nil {
		LOGE.Fatalln("Failed to setup StwServer:", err)
	}

	// Run tests.
	for _, t := range tests {
		if b, err := regexp.MatchString(*testRegex, t.name); b && err == nil {
			fmt.Printf("Running %s:\n", t.name)
			t.f()
		}
	}

	fmt.Printf("Passed (%d/%d) tests\n", passCount, passCount+failCount)
}
