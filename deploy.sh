#!/bin/bash

if [ -z $GOPATH ]; then
    echo "FAIL: GOPATH environment variable is not set"
    exit 1
fi

if [ -n "$(go version | grep 'darwin/amd64')" ]; then    
    GOOS="darwin_amd64"
elif [ -n "$(go version | grep 'linux/amd64')" ]; then
    GOOS="linux_amd64"
else
    echo "FAIL: only 64-bit Mac OS X and Linux operating systems are supported"
    exit 1
fi

go install runners/rwebserver
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi

go install runners/rstwserver
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi

go install runners/rstorage
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi

# Pick random port between [10000, 20000).
STORAGE_PORT=$(((RANDOM % 10000) + 10000))
STORAGE_SERVER=$GOPATH/bin/rstorage

# Pick random port between [20000, 30000).
STW_PORT=$(((RANDOM % 10000) + 20000))
STW_SERVER=$GOPATH/bin/rstwserver

WEB_PORT=8080
WEB_SERVER=$GOPATH/bin/rwebserver

STORAGE_ID=('0' '0' '0' '0' '0') # assign random id if set 0
STW_SERVER_NUM=5

function startStorageServers {
    N=${#STORAGE_ID[@]}
    # Start master storage server.
    ${STORAGE_SERVER} -N=${N} -id=${STORAGE_ID[0]} -port=${STORAGE_PORT}  &
    STORAGE_SERVER_PID[0]=$!
    # Start slave storage servers.
    if [ "$N" -gt 1 ]
    then
        for i in `seq 1 $((N - 1))`
        do
	    	STORAGE_SLAVE_PORT=$(((RANDOM % 10000) + 10000))
            ${STORAGE_SERVER} -port=${STORAGE_SLAVE_PORT} -id=${STORAGE_ID[$i]} -master="localhost:${STORAGE_PORT}"  &
            STORAGE_SERVER_PID[$i]=$!
        done
    fi
    sleep 5
}

function stopStorageServers {
    N=${#STORAGE_ID[@]}
    for i in `seq 0 $((N - 1))`
    do
        kill -9 ${STORAGE_SERVER_PID[$i]}
        wait ${STORAGE_SERVER_PID[$i]} 2> /dev/null
    done
}

function startStwServers {
	N=${STW_SERVER_NUM}
	# Start master stwserver.
    ${STW_SERVER} -N=${N} -port=${STW_PORT} -storageMaster="localhost:${STORAGE_PORT}" &
    STW_SERVER_PID[0]=$!
    if [ "$N" -gt 1 ]
    then
	    for i in `seq 1 $((N - 1))`
	    do
	        # Pick random port between [20000, 30000).
	        STW_PORT[$i]=$(((RANDOM % 10000) + 20000))
	        ${STW_SERVER} -port=${STW_PORT[$i]} -master="localhost:${STW_PORT}" -storageMaster="localhost:${STORAGE_PORT}" &
	        STW_SERVER_PID[$i]=$!
	    done
	fi
    sleep 5
}

function stopStwServers {
	N=${STW_SERVER_NUM}
    for i in `seq 0 $((N - 1))`
    do
        kill -9 ${STW_SERVER_PID[$i]}
        wait ${STW_SERVER_PID[$i]} 2> /dev/null
    done
}

function main {
	echo "Starting ${#STORAGE_ID[@]} storage server(s)..."
    startStorageServers
    echo "Starting ${STW_SERVER_NUM} app server(s)..."
    startStwServers
	echo "Starting web server..."
    ${WEB_SERVER} -port=${WEB_PORT} -masterApp="localhost:${STW_PORT}"
    WEB_SERVER_PID=$!

    kill -9 ${WEB_SERVER_PID}
    wait ${WEB_SERVER_PID} 2> /dev/null

    stopStwServers
    stopStorageServers
    sleep 1
}

main


