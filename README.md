## SimpleTwitter

<img src="https://users.soe.ucsc.edu/~lwang107/userfiles/images/document/design_scale.png" width="70%">

A simple but scalable twitter-like information dessemination service (call it SimpleTwitter for short) that provides a minimum set of functionality including posting tweet, deleting tweet, subscribing/unsubscribing other users and viewing subscribed users' tweets in timeline fashion. A running instance is deployed as a [Demo](http://simpletwitter.liang-w.xyz/).

The backend of this project is an implementation to [https://github.com/CMU-440-F16/p2](https://github.com/CMU-440-F16/p2) (with some modifications).

The code is organized as follows:

```
deploy.sh                          Deploy system on localhost:8080

bin/                               Compiled binaries

client/                            Front-end of SimpleTwitter

src/
  webserver/                       Web server of SimpleTwitter
  stwclient/                       Client of stwserver
  stwserver/                       Application Server
  libstore/                        Client of storage server as a library
  storageserver/                   Key-value storage server

  runners/                         Main functions that run servers

  util/                            Util functions
    keyFormatter.go                Format/parse the key posted to storage server
    common.go                      Assistant functions

  tests/                           Source code for official tests
    proxycounter/                  Utility package used by the official tests
    stwtest/                       Tests the StwServer
    libtest/                       Tests the Libstore
    storagetest/                   Tests the StorageServer
    stresstest/                    Tests everything
  
  rpc/
    stwrpc/                        StwServer RPC helpers/constants
    librpc/                        Libstore RPC helpers/constants
    storagerpc/                    StorageServer RPC helpers/constants
    
tests/                             Shell scripts to run the tests
```

To deploy system on localhost, type

```
./deoply.sh
```

To run the tests, type

```
$GOPATH/tests/runall.sh
```

To start up each server manually, you can first

```
go install github.com/runners/rstorage
go install github.com/runners/rstwserver
go install github.com/runners/rwebserver
```

then

```
$GOPATH/bin/rstorage -port=${STORAGE_PORT}
$GOPATH/bin/rstwserver -port=${STW_PORT} -storageMaster="localhost:${STORAGE_PORT}"
$GOPATH/bin/rwebserver -masterApp="localhost:${STW_PORT}"
```

Parameter specifications of each server can be found in `$GOPATH/src/runners`.
