/*
 * BSD-3-Clause
 * Copyright 2020 sot (PR_713, C_rho_272)
 * Redistribution and use in source and binary forms, with or without modification,
 * are permitted provided that the following conditions are met:
 * 1. Redistributions of source code must retain the above copyright notice,
 * this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright notice,
 * this list of conditions and the following disclaimer in the documentation and/or
 * other materials provided with the distribution.
 * 3. Neither the name of the copyright holder nor the names of its contributors
 * may be used to endorse or promote products derived from this software without
 * specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
 * WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
 * IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT,
 * INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING,
 * BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA,
 * OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY,
 * WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
 * ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY
 * OF SUCH DAMAGE.
 */

package shared

import (
	"bytes"
	"errors"
	"github.com/nfnt/resize"
	"github.com/zeebo/bencode"
	"image"
	"image/jpeg"
	"io/ioutil"
	"net/http"
	"path/filepath"
)

type TorrentInfo struct {
	Id     int64
	Name   string
	URL    string
	Image  []byte
	Meta   map[string]string
	Files  map[string]bool
	Length uint64
}

func (t TorrentInfo) NewFiles() []string {
	var res []string
	for file, isNew := range t.Files {
		if isNew {
			res = append(res, file)
		}
	}
	return res
}

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

func GetTorrent(url string) (*TorrentInfo, error) {
	var res *TorrentInfo
	var err error
	if resp, httpErr := http.Get(url); httpErr == nil && resp != nil && resp.StatusCode < 400 {
		defer resp.Body.Close()
		var torrent Torrent
		err := bencode.NewDecoder(resp.Body).Decode(&torrent)
		if err == nil {
			res = &TorrentInfo{
				Name:  torrent.Info.Name,
				URL:   torrent.PublisherUrl,
				Files: make(map[string]bool),
			}
			if torrent.Info.Files != nil {
				for _, file := range torrent.Info.Files {
					if file.Path != nil {
						allParts := []string{torrent.Info.Name}
						allParts = append(allParts, file.Path...)
						res.Files["/"+filepath.Join(allParts...)] = true
					}
					res.Length += file.Length
				}
			} else {
				res.Files["/"+torrent.Info.Name] = true
				res.Length = torrent.Info.Length
			}
		}
	} else {
		errMsg := "crawling: "
		if httpErr != nil {
			errMsg += httpErr.Error()
		} else if resp == nil {
			errMsg += "empty response"
		} else {
			errMsg += resp.Status
		}
		err = errors.New(errMsg)
	}
	return res, err
}


func GetTorrentPoster(imageUrl string, maxSize uint) (error, []byte) {
	var err error
	var torrentImage []byte
	if len(imageUrl) > 0 {
		if resp, httpErr := http.Get(imageUrl); httpErr == nil && resp != nil && resp.StatusCode < 400 {
			defer resp.Body.Close()
			var img image.Image
			if img, _, err = image.Decode(resp.Body); err == nil && img != nil {
				if maxSize > 0 {
					img = resize.Thumbnail(maxSize, maxSize, img, resize.Bicubic)
				}
				imgBuffer := bytes.Buffer{}
				if err = jpeg.Encode(&imgBuffer, img, nil); err == nil {
					torrentImage, err = ioutil.ReadAll(&imgBuffer)
				}
			}
		} else {
			errMsg := "crawling: "
			if httpErr != nil {
				errMsg += httpErr.Error()
			} else if resp == nil {
				errMsg += "empty response"
			} else {
				errMsg += resp.Status
			}
			err = errors.New(errMsg)
		}
	} else {
		err = errors.New("invalid image url")
	}
	return err, torrentImage
}