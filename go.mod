module sot-te.ch/TTObserverV1

go 1.13

require (
	github.com/azzzak/vkapi v0.0.0-20190905132831-5fc550e1c8f4
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/zeebo/bencode v1.0.0
	sot-te.ch/HTExtractor v0.1.0
	sot-te.ch/MTHelper v0.1.9
)

replace sot-te.ch/MTHelper => ../MTHelper

replace sot-te.ch/HTExtractor => ../HTExtractor
