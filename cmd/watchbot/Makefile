default:	secbot

secbot:	*.go ../../watchbot/*.go ../../telegram/*.go ../../db/*.go
	GOPATH=$(realpath ../../)
	go get github.com/op/go-logging
	go build