module sot-te.ch/TTObserverV1

go 1.24.0

toolchain go1.24.2

require (
	github.com/PowerDNS/lmdb-go v1.9.3
	github.com/azzzak/vkapi v0.0.0-20190905132831-5fc550e1c8f4
	github.com/go-resty/resty/v2 v2.16.5
	github.com/lib/pq v1.10.9
	github.com/mattn/go-sqlite3 v1.14.32
	github.com/nats-io/nats.go v1.47.0
	github.com/nats-io/stan.go v0.10.4
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/redis/go-redis/v9 v9.17.0
	github.com/zeebo/bencode v1.0.0
	golang.org/x/image v0.33.0
	sot-te.ch/GoHTExtractor v0.1.3
	sot-te.ch/GoMTHelper v0.2.6
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/klauspost/compress v1.18.1 // indirect
	github.com/nats-io/nats-server/v2 v2.11.1 // indirect
	github.com/nats-io/nats-streaming-server v0.25.6 // indirect
	github.com/nats-io/nkeys v0.4.11 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/stretchr/testify v1.10.0 // indirect
	github.com/xlzd/gotp v0.1.0 // indirect
	github.com/zelenin/go-tdlib v0.7.6 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/time v0.11.0 // indirect
)

replace sot-te.ch/GoMTHelper => ../mthelper

replace sot-te.ch/GoHTExtractor => ../htextractor
