package intl

import (
	"github.com/zeebo/bencode"
	"io"
)

type Torrent struct {
	AnnounceList [][]string `bencode:"announce-list"`
	Announce     string     `bencode:"announce"`
	Comment      string     `bencode:"comment"`
	CreatedBy    string     `bencode:"created by"`
	CreationDate int64      `bencode:"creation date"`
	Publisher    string     `bencode:"publisher"`
	PublisherUrl string     `bencode:"publisher-url"`
	Encoding     string     `bencode:"encoding"`
	Info         struct {
		Length uint64 `bencode:"length"`
		Files  []struct {
			Length uint64   `bencode:"length"`
			Path   []string `bencode:"path"`
		} `bencode:"files"`
		Name        string `bencode:"name"`
		PieceLength uint64 `bencode:"piece length"`
		Pieces      []byte `bencode:"pieces"`
	} `bencode:"info"`
}

func ReadTorrent(stream io.Reader) (*Torrent, error) {
	var torrent Torrent
	err := bencode.NewDecoder(stream).Decode(&torrent)
	return &torrent, err
}

func (t *Torrent) FullLength() uint64 {
	var fullLen uint64
	if t.Info.Length > 0 {
		fullLen = t.Info.Length
	} else {
		if t.Info.Files != nil {
			for _, file := range t.Info.Files {
				fullLen += file.Length
			}
		}
	}
	return fullLen * t.Info.PieceLength
}
