module sot-te.ch/TTObserverV1

go 1.13

require (
	github.com/azzzak/vkapi v0.0.0-20190905132831-5fc550e1c8f4
	github.com/go-resty/resty/v2 v2.7.0 // indirect
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/mattn/go-sqlite3 v1.14.9
	github.com/nats-io/nats-server/v2 v2.4.0 // indirect
	github.com/nats-io/nats-streaming-server v0.22.1 // indirect
	github.com/nats-io/nats.go v1.13.0
	github.com/nats-io/stan.go v0.10.2
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/zeebo/bencode v1.0.0
	golang.org/x/crypto v0.0.0-20211117183948-ae814b36b871 // indirect
	golang.org/x/net v0.0.0-20211118161319-6a13c67c3ce4 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	sot-te.ch/HTExtractor v0.1.2
	sot-te.ch/MTHelper v0.1.13
)

replace sot-te.ch/MTHelper => ../MTHelper

replace sot-te.ch/HTExtractor => ../HTExtractor
