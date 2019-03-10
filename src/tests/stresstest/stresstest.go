package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"rpc/stwrpc"
	"stwclient"
)

const (
	Subscribe = iota
	Unsubscribe
	Timeline
	Post
	HomeTimeline
)

var (
	portnum  = flag.Int("port", 9010, "server port # to connect to")
	clientId = flag.String("clientId", "0", "client id for user")
	numCmds  = flag.Int("numCmds", 1000, "number of random commands to execute")
	seed     = flag.Int64("seed", 0, "seed for random number generator used to execute commands")
)

var LOGE = log.New(os.Stderr, "", log.Lshortfile|log.Lmicroseconds)

var statusMap = map[stwrpc.Status]string{
	stwrpc.OK:               "OK",
	stwrpc.NoSuchUser:       "NoSuchUser",
	stwrpc.NoSuchTargetUser: "NoSuchTargetUser",
	stwrpc.Exists:           "Exists",
	0:                       "Unknown",
}

var (
	// Debugging information (counts the total number of operations performed).
	fs, as, rs, gt, pt, gtbs int
	// Set this to true to print debug information.
	debug bool
)

func main() {
	flag.Parse()
	if flag.NArg() < 2 {
		LOGE.Fatalln("Usage: ./stressclient <user> <numTargets>")
	}

	client, err := stwclient.NewStwClient("localhost", *portnum)
	if err != nil {
		LOGE.Fatalln("FAIL: NewStwClient returned error:", err)
	}

	user := flag.Arg(0)
	userNum, err := strconv.Atoi(user)
	if err != nil {
		LOGE.Fatalf("FAIL: user %s not an integer\n", user)
	}
	numTargets, err := strconv.Atoi(flag.Arg(1))
	if err != nil {
		LOGE.Fatalf("FAIL: numTargets invalid %s\n", flag.Arg(1))
	}

	_, err = client.CreateUser(user)
	if err != nil {
		LOGE.Fatalf("FAIL: error when creating userID '%s': %s\n", user, err)
	}

	stwIndex := 0
	if *seed == 0 {
		rand.Seed(time.Now().UnixNano())
	} else {
		rand.Seed(*seed)
	}

	cmds := make([]int, *numCmds)
	for i := 0; i < *numCmds; i++ {
		cmds[i] = rand.Intn(6)
		switch cmds[i] {
		case Subscribe:
			as++
		case Unsubscribe:
			rs++
		case Timeline:
			gt++
		case Post:
			pt++
		case HomeTimeline:
			gtbs++
		}
	}

	if debug {
		// Prints out the total number of operations that will be performed.
		fmt.Println("Subscribe:", as)
		fmt.Println("Unsubscribe:", rs)
		fmt.Println("Timeline:", gt)
		fmt.Println("Post:", pt)
		fmt.Println("HomeTimeline:", gtbs)
	}
	t := time.Now()
	old_cmd :=cmds[0]
	for _, cmd := range cmds {
		if time.Now().Sub(t)>100*time.Millisecond {
			fmt.Println(old_cmd, "is Slow")
		}
		t = time.Now()
		old_cmd = cmd
		switch cmd {
		case Subscribe:
			target := rand.Intn(numTargets)
			status, err := client.Subscribe(user, strconv.Itoa(target))
			if err != nil {
				LOGE.Fatalf("FAIL: Subscribe returned error '%s'\n", err)
			}
			if status == 0 || status == stwrpc.NoSuchUser {
				LOGE.Fatalf("FAIL: Subscribe returned error status '%s'\n", statusMap[status])
			}
		case Unsubscribe:
			target := rand.Intn(numTargets)
			status, err := client.Unsubscribe(user, strconv.Itoa(target))
			if err != nil {
				LOGE.Fatalf("FAIL: Unsubscribe returned error '%s'\n", err)
			}
			if status == 0 || status == stwrpc.NoSuchUser {
				LOGE.Fatalf("FAIL: Unsubscribe returned error status '%s'\n", statusMap[status])
			}
		case Timeline:
			target := rand.Intn(numTargets)
			posts, status, err := client.Timeline(strconv.Itoa(target))
			if err != nil {
				LOGE.Fatalf("FAIL: Timeline returned error '%s'\n", err)
			}
			if status == 0 {
				LOGE.Fatalf("FAIL: Timeline returned error status '%s'\n", statusMap[status])
			}
			if !validatePosts(&posts, numTargets) {
				fmt.Println(&posts, numTargets)
				LOGE.Fatalln("FAIL: failed while validating returned posts")
			}
		case Post:
			stwVal := userNum + stwIndex*numTargets
			msg := fmt.Sprintf("%d;%s", stwVal, *clientId)
			reply, err := client.Post(user, msg)
			if err != nil {
				LOGE.Fatalf("FAIL: Post returned error '%s'\n", err)
			}
			if reply.Status == 0 || reply.Status == stwrpc.NoSuchUser {
				LOGE.Fatalf("FAIL: Post returned error status '%s'\n",
					statusMap[reply.Status])
			}
			stwIndex++
		case HomeTimeline:
			posts, status, err := client.HomeTimeline(user)
			if err != nil {
				LOGE.Fatalf("FAIL: HomeTimeline returned error '%s'\n", err)
			}
			if status == 0 || status == stwrpc.NoSuchUser {
				LOGE.Fatalf("FAIL: HomeTimeline returned error status '%s'\n", statusMap[status])
			}
			if !validatePosts(&posts, numTargets) {
				LOGE.Fatalln("FAIL: failed while validating returned posts")
			}
		}
	}
	fmt.Println("PASS")
	os.Exit(7)
}

/*func validateSubscriptions(subscriptions *[]string) bool {
	subscriptionSet := make(map[string]bool, len(*subscriptions))
	for _, subscription := range *subscriptions {
		if subscriptionSet[subscription] == true {
			return false
		}
		subscriptionSet[subscription] = true
	}
	return true
}*/

func validatePosts(posts *[]stwrpc.Post, numTargets int) bool {
	userIdToLastVal := make(map[string]int, len(*posts))
	for _, post := range *posts {
		valAndId := strings.Split(post.Contents, ";")
		val, err := strconv.Atoi(valAndId[0])
		if err != nil {
			return false
		}
		user, err := strconv.Atoi(post.UserID)
		if err != nil {
			return false
		}
		userClientId := fmt.Sprintf("%s;%s", post.UserID, valAndId[1])
		lastVal := userIdToLastVal[userClientId]
		if val%numTargets == user && (lastVal == 0 || lastVal == val+numTargets || lastVal == val) {
			userIdToLastVal[userClientId] = val
		} else {
			fmt.Println("invalid:", lastVal, val)
			return false
		}
	}
	return true
}
