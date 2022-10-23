module sot-te.ch/TTObserverV1

go 1.18

require (
	github.com/azzzak/vkapi v0.0.0-20190905132831-5fc550e1c8f4
	github.com/go-redis/redis/v8 v8.11.5
	github.com/lib/pq v1.10.7
	github.com/mattn/go-sqlite3 v1.14.15
	github.com/minio/sha256-simd v1.0.0
	github.com/nats-io/nats.go v1.18.0
	github.com/nats-io/stan.go v0.10.3
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/zeebo/bencode v1.0.0
	golang.org/x/image v0.1.0
	sot-te.ch/HTExtractor v0.1.3
	sot-te.ch/MTHelper v0.2.2
)

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-resty/resty/v2 v2.7.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/klauspost/cpuid/v2 v2.1.2 // indirect
	github.com/nats-io/nats-server/v2 v2.7.4 // indirect
	github.com/nats-io/nats-streaming-server v0.24.3 // indirect
	github.com/nats-io/nkeys v0.3.0 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/xlzd/gotp v0.0.0-20220915034741-1546cf172da8 // indirect
	github.com/zelenin/go-tdlib v0.6.0 // indirect
	golang.org/x/crypto v0.1.0 // indirect
	golang.org/x/net v0.1.0 // indirect
	golang.org/x/sys v0.1.0 // indirect
)

replace sot-te.ch/MTHelper => ../MTHelper

replace sot-te.ch/HTExtractor => ../HTExtractor
