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
	"crypto/sha1"
	"errors"
	"hash"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/minio/sha256-simd"
	"github.com/nfnt/resize"
	"github.com/zeebo/bencode"
	_ "golang.org/x/image/webp"
)

var ErrInvalidImageURL = errors.New("invalid image url")

type TorrentInfo struct {
	Id     int64
	Name   string
	URL    string
	Image  []byte
	Meta   map[string]string
	Files  map[string]bool
	Data   []byte
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
	var resp *http.Response
	if resp, err = http.Head(url); err == nil && resp != nil { // nolint:gosec
		resp.Close = true
		_ = resp.Body.Close()
		if resp.StatusCode < 400 {
			var data []byte
			if resp, err = http.Get(url); err == nil && resp != nil && resp.StatusCode < 400 { // nolint:gosec
				resp.Close = true
				defer resp.Body.Close()
				if data, err = io.ReadAll(resp.Body); err == nil {
					torrent := new(Torrent)
					if err = bencode.DecodeBytes(data, torrent); err == nil {
						res = &TorrentInfo{
							Name:  torrent.Info.Name,
							URL:   torrent.PublisherUrl,
							Files: make(map[string]bool),
							Data:  data,
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
				}
			} else {
				err = buildError(resp, err, "get torrent")
			}
		}
	}

	return res, err
}

func GetTorrentPoster(imageUrl string, maxSize uint) ([]byte, error) {
	var err error
	var torrentImage []byte
	if len(imageUrl) > 0 {
		if resp, httpErr := http.Get(imageUrl); httpErr == nil && resp != nil && resp.StatusCode < 400 { // nolint:gosec
			resp.Close = true
			defer resp.Body.Close()
			if maxSize > 0 {
				var img image.Image
				if img, _, err = image.Decode(resp.Body); err == nil && img != nil {
					if maxSize > 0 {
						img = resize.Thumbnail(maxSize, maxSize, img, resize.Bicubic)
					}
					imgBuffer := new(bytes.Buffer)
					if err = jpeg.Encode(imgBuffer, img, &jpeg.Options{Quality: 90}); err == nil {
						torrentImage = imgBuffer.Bytes()
					}
				}
			} else {
				torrentImage, err = io.ReadAll(resp.Body)
			}
		} else {
			err = buildError(resp, httpErr, "get poster")
		}
	} else {
		err = ErrInvalidImageURL
	}
	return torrentImage, err
}

func buildError(resp *http.Response, httpErr error, desc string) error {
	sb := strings.Builder{}
	sb.WriteString(desc)
	if sb.Len() > 0 {
		sb.WriteRune('.')
	}
	if httpErr != nil {
		sb.WriteString(httpErr.Error())
		sb.WriteRune(' ')
	}
	if resp == nil {
		sb.WriteString("empty response")
	} else {
		resp.Close = true
		_ = resp.Body.Close()
		sb.WriteString(resp.Status)
	}
	return errors.New(sb.String())
}

type BencodeRawBytes []byte

func (ba *BencodeRawBytes) UnmarshalBencode(in []byte) error {
	*ba = append([]byte(nil), in...)
	return nil
}

type torrentRawInfoStruct struct {
	Info BencodeRawBytes `bencode:"info"`
}

func GenerateTorrentInfoHash(data []byte, v2 bool) (h1, h2 []byte, err error) {
	torrent := new(torrentRawInfoStruct)
	if err = bencode.DecodeBytes(data, torrent); err == nil {
		var h hash.Hash
		h = sha1.New()
		h.Write(torrent.Info)
		h1 = h.Sum(nil)
		if v2 {
			h = sha256.New()
			h.Write(torrent.Info)
			h2 = h.Sum(nil)
		}
	}
	return
}
