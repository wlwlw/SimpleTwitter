package util

import (
	"strconv"
	"strings"
	
	"libstore"
)

// type ByAlphabet []string

// func (s ByAlphabet) Len() int { return len(s) }

// func (s ByAlphabet) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// func (s ByAlphabet) Less(i, j int) bool {
// 	var si string = s[i]
//     var sj string = s[j]
//     return si < sj
// }

func BinarySearchString(vals []string, val string) int {
	i, j, mid := 0, len(vals)-1, 0
	for ; i<=j; {
		mid = (i+j)/2
		if vals[mid]==val {
			return mid
		} else if vals[mid] < val {
			i = mid+1
		} else {
			j = mid-1
		}
	}
	return i
}

func BinarySearchUint32(vals []uint32, val uint32) int {
	i, j, mid := 0, len(vals)-1, 0
	for ; i<=j; {
		mid = (i+j)/2
		if vals[mid]==val {
			return mid
		} else if vals[mid] < val {
			i = mid+1
		} else {
			j = mid-1
		}
	}
	return i
}

func SearchHashRing(ring []uint32, key string) uint32 {
	hash := libstore.StoreHash(key)
	i := BinarySearchUint32(ring, hash)
	if i == len(ring) {
		i = 0
	}
	return ring[i]
}

func FormatLeaseRecord(hostport string, time int64) string {
	return hostport+"~"+strconv.FormatInt(time, 16)
}

func ParseLeaseRecord(leaseRecord string) (hostport string, time int64) {
	slist := strings.Split(leaseRecord, "~")
	hostport, res := slist[0], slist[1]
	time, _ = strconv.ParseInt(res, 16, 64)
	return hostport, time
}

func BinarySearchLeaseRecord(vals []string, val string) int {
	i, j, mid := 0, len(vals)-1, 0
	val, _ = ParseLeaseRecord(val)
	for ; i<=j; {
		mid = (i+j)/2
		tar, _ := ParseLeaseRecord(vals[mid])
		if tar==val {
			return mid
		} else if tar < val {
			i = mid+1
		} else {
			j = mid-1
		}
	}
	return i
}
