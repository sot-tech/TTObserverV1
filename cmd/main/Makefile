default:	ttobserver

ttobserver:	*.go ../../observer ../../intl/*.go
	GOPATH=$(realpath ../../)
	go get github.com/op/go-logging github.com/zeebo/bencode github.com/go-telegram-bot-api/telegram-bot-api
	go build