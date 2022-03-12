module sot-te.ch/TTObserverV1

go 1.13

require (
	github.com/azzzak/vkapi v0.0.0-20190905132831-5fc550e1c8f4
	github.com/go-redis/redis/v8 v8.11.4
	github.com/go-resty/resty/v2 v2.7.0 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/klauspost/cpuid/v2 v2.0.11 // indirect
	github.com/mattn/go-sqlite3 v1.14.12
	github.com/minio/sha256-simd v1.0.0
	github.com/nats-io/nats-streaming-server v0.24.3 // indirect
	github.com/nats-io/nats.go v1.13.1-0.20220308171302-2f2f6968e98d
	github.com/nats-io/stan.go v0.10.2
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/stretchr/testify v1.5.1
	github.com/zeebo/bencode v1.0.0
	golang.org/x/image v0.0.0-20220302094943-723b81ca9867
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f // indirect
	sot-te.ch/HTExtractor v0.1.2
	sot-te.ch/MTHelper v0.2.0
)

replace sot-te.ch/MTHelper => ../MTHelper

replace sot-te.ch/HTExtractor => ../HTExtractor
