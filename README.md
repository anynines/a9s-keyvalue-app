# a9s KeyValue App

This is a sample app to check whether the a9s KeyValue Service is working or not.

## Install, Push and Bind

Make sure you installed Go on your machine, [download this](https://go.dev/dl/go1.23.4.darwin-amd64.pkg) for macOS.

Download the application:

```shell
go get github.com/anynines/a9s-keyvalue-app
cd $GOPATH/src/github.com/anynines/a9s-keyvalue-app
```

Create a service on the [a9s PaaS](https://paas.anynines.com):

```shell
cf create-service a9s-keyvalue keyvalue8-single-ssl my-keyvalue-service
```

Push the app:

```shell
cf push --no-start
```

Bind the app:

```shell
cf bind-service keyvalue-app my-keyvalue-service
```

And start:

```shell
cf start keyvalue-app
```

At last, check the created url...

## Local Test

Start Valkey service with Docker:

```shell
docker run -d -p 6379:6379 valkey/valkey valkey-server --requirepass secret
```

Export a few environment variables and run the sample app:

```shell
export VALKEY_HOST=localhost
export VALKEY_PORT=6379
export VALKEY_PASSWORD=secret
export VALKEY_USERNAME=default
export APP_DIR=$PWD
go build
./a9s-keyvalue-app
```

## Remark

To bind the app to other KeyValue services than `a9s-keyvalue`, have a look at the `VCAPServices` struct.
