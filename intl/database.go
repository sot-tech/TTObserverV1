package intl

type Database struct {
	Path string
}

func (db *Database) GetChats() []int64 {
	return []int64{}
}

func (db *Database) InsertChat(chat int64) {

}

func (db *Database) DeleteChat(chat int64) {

}

func (db *Database) GetTorrentLength(torrent string) uint64 {
	return 0
}

func (db *Database) UpsertTorrent(name string, length uint64) {

}

func (db *Database) GetCrawlOffset() uint {
	return 0
}

func (db *Database) UpsertCrawlOffset(offset uint){

}

func (db *Database) GetTgOffset() int {
	return 0
}

func (db *Database) UpsertTgOffset(offset int){

}

