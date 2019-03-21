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
go install runners/rwebserver
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi
go install tests/webstresstest
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi

# Pick random port between [10000, 20000).
STORAGE_PORT=$(((RANDOM % 10000) + 10000))
STORAGE_SERVER=$GOPATH/bin/rstorage
STRESS_CLIENT=$GOPATH/bin/webstresstest
STW_PORT=$(((RANDOM % 10000) + 20000))
STW_SERVER=$GOPATH/bin/rstwserver

WEB_PORT=8080
WEB_SERVER=$GOPATH/bin/rwebserver

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

function startWebServer {
    ${WEB_SERVER} -port=${WEB_PORT} -masterApp="localhost:${STW_PORT}" &
    WEB_SERVER_PID=$!
    sleep 1
}

function stopWebServer {
    kill -9 ${WEB_SERVER_PID}
    wait ${WEB_SERVER_PID} 2> /dev/null
}

function testStress {
    echo "Starting ${#STORAGE_ID[@]} storage server(s)..."
    startStorageServers
    echo "Starting ${STW_SERVER_NUM} App server(s)..."
    startStwServers
    echo "Starting web server..."
    startWebServer

    # Start stress clients
    C=0
    K=${#CLIENT_COUNT[@]}
    for USER in `seq 0 $((K - 1))`
    do
        for CLIENT in `seq 0 $((CLIENT_COUNT[$USER] - 1))`
        do
            ${STRESS_CLIENT} -port=${WEB_PORT} -clientId=${CLIENT} ${USER} ${K} & 
            STRESS_CLIENT_PID[$C]=$!
            # Setup background thread to kill client upon timeout.
            sleep ${TIMEOUT} && kill -9 ${STRESS_CLIENT_PID[$C]} &>/dev/null &
            C=$((C + 1))
        done
    done
    echo "Running ${C} client(s)..."

    # Check exit status.
    FAIL=0
    for i in `seq 0 $((C - 1))`
    do
        wait ${STRESS_CLIENT_PID[$i]} 
        if [ "$?" -ne 7 ]
        then
            FAIL=$((FAIL + 1))
        fi
    done
    if [ "$FAIL" -eq 0 ]
    then
        echo "PASS"
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        echo "FAIL: ${FAIL} clients failed"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
    stopWebServer
    stopStwServers
    stopStorageServers
    sleep 1
}

# Testing single client, single tribserver, single storageserver.
function testStressSingleClientSingleStwSingleStorage {
    echo "Running testStressSingleClientSingleStwSingleStorage:"
    STORAGE_ID=('0')
    STW_SERVER_NUM=1
    CLIENT_COUNT=('1')
    TIMEOUT=15
    testStress
}

# Testing single client, single tribserver, multiple storageserver.
function testStressSingleClientSingleStwMultipleStorage {
    echo "Running testStressSingleClientSingleStwMultipleStorage:"
    STORAGE_ID=('0' '0' '0')
    STW_SERVER_NUM=1
    CLIENT_COUNT=('1')
    TIMEOUT=15
    testStress
}

# Testing multiple client, single tribserver, single storageserver.
function testStressMultipleClientSingleStwSingleStorage {
    echo "Running testStressMultipleClientSingleStwSingleStorage:"
    STORAGE_ID=('0')
    STW_SERVER_NUM=1
    CLIENT_COUNT=('1' '1' '1')
    TIMEOUT=15
    testStress
}

# Testing multiple client, single tribserver, multiple storageserver.
function testStressMultipleClientSingleStwMultipleStorage {
    echo "Running testStressMultipleClientSingleStwMultipleStorage:"
    STORAGE_ID=('0' '0' '0' '0' '0' '0')
    STW_SERVER_NUM=1
    CLIENT_COUNT=('1' '1' '1')
    TIMEOUT=15
    testStress
}

# Testing multiple client, multiple tribserver, single storageserver.
function testStressMultipleClientMultipleStwSingleStorage {
    echo "Running testStressMultipleClientMultipleStwSingleStorage:"
    STORAGE_ID=('0')
    STW_SERVER_NUM=2
    CLIENT_COUNT=('1' '1')
    TIMEOUT=30
    testStress
}

# Testing multiple client, multiple tribserver, multiple storageserver.
function testStressMultipleClientMultipleStwMultipleStorage {
    echo "Running testStressMultipleClientMultipleStwMultipleStorage:"
    STORAGE_ID=('0' '0' '0' '0' '0' '0' '0')
    STW_SERVER_NUM=3
    CLIENT_COUNT=('1' '1' '1')
    TIMEOUT=30
    testStress
}

# Testing 2x more clients than tribservers, multiple tribserver, multiple storageserver.
function testStressDoubleClientMultipleStwMultipleStorage {
    echo "Running testStressDoubleClientMultipleStwMultipleStorage:"
    STORAGE_ID=('0' '0' '0' '0' '0' '0')
    STW_SERVER_NUM=2
    CLIENT_COUNT=('1' '1' '1' '1')
    TIMEOUT=30
    testStress
}


# Testing duplicate users, multiple tribserver, single storageserver.
function testStressDupUserMultipleStwSingleStorage {
    echo "Running testStressDupUserMultipleStwSingleStorage:"
    STORAGE_ID=('0')
    STW_SERVER_NUM=2
    CLIENT_COUNT=('2')
    TIMEOUT=30
    testStress
}

# Testing duplicate users, multiple tribserver, multiple storageserver.
function testStressDupUserMultipleStwMultipleStorage {
    echo "Running testStressDupUserMultipleStwMultipleStorage:"
    STORAGE_ID=('0' '0' '0')
    STW_SERVER_NUM=2
    CLIENT_COUNT=('2')
    TIMEOUT=30
    testStress
}

function testHugeLoadPerformance1 {
    echo "Running testHugeLoadPerformance1:"
    STORAGE_ID=('0')
    STW_SERVER_NUM=1
    CLIENT_COUNT=('10')
    TIMEOUT=30
    testStress
}

function testHugeLoadPerformance2 {
    echo "Running testHugeLoadPerformance2:"
    STORAGE_ID=('0' '0')
    STW_SERVER_NUM=2
    CLIENT_COUNT=('10')
    TIMEOUT=30
    testStress
}

function testHugeLoadPerformance3 {
    echo "Running testtestHugeLoadPerformance3:"
    STORAGE_ID=('0' '0' '0')
    STW_SERVER_NUM=3
    CLIENT_COUNT=('10')
    TIMEOUT=30
    testStress
}

function testHugeLoadPerformance4 {
    echo "Running testtestHugeLoadPerformance4:"
    STORAGE_ID=('0' '0' '0' '0' '0')
    STW_SERVER_NUM=5
    CLIENT_COUNT=('10')
    TIMEOUT=30
    testStress
}

function testHugeLoadPerformance5 {
    echo "Running testtestHugeLoadPerformance5:"
    STORAGE_ID=('0' '0' '0' '0' '0' '0' '0' '0' '0' '0')
    STW_SERVER_NUM=10
    CLIENT_COUNT=('10')
    TIMEOUT=30
    testStress
}

# Run tests.
PASS_COUNT=0
FAIL_COUNT=0
# testStressSingleClientSingleStwSingleStorage
# testStressSingleClientSingleStwMultipleStorage
# testStressMultipleClientSingleStwSingleStorage
# testStressMultipleClientSingleStwMultipleStorage
# testStressMultipleClientMultipleStwSingleStorage
# testStressMultipleClientMultipleStwMultipleStorage
# testStressDoubleClientMultipleStwMultipleStorage
# testStressDupUserMultipleStwSingleStorage
# testStressDupUserMultipleStwMultipleStorage

echo "Warning: huge load test better run one by one if running on single host"

# testHugeLoadPerformance1
# testHugeLoadPerformance2
testHugeLoadPerformance3
testHugeLoadPerformance4
testHugeLoadPerformance5

echo "Passed (${PASS_COUNT}/$((PASS_COUNT + FAIL_COUNT))) tests"
