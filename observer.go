/*
 * BSD-3-Clause
 * Copyright 2020 sot (aka PR_713, C_rho_272)
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

package TTObserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nfnt/resize"
	"github.com/op/go-logging"
	"html"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sot-te.ch/HTExtractor"
	tg "sot-te.ch/MTHelper"
	"strings"
	tmpl "text/template"
	"time"
)

type Observer struct {
	Log struct {
		File  string `json:"file"`
		Level string `json:"level"`
	} `json:"log"`
	Crawler struct {
		BaseURL        string                      `json:"baseurl"`
		ContextURL     string                      `json:"contexturl"`
		Delay          uint                        `json:"delay"`
		Threshold      uint                        `json:"threshold"`
		Anniversary    uint                        `json:"anniversary"`
		MetaActions    []HTExtractor.ExtractAction `json:"metaactions"`
		ImageMetaField string                      `json:"imagemetafield"`
		ImageThumb     uint                        `json:"imagethumb"`
		MetaExtractor  *HTExtractor.Extractor      `json:"-"`
	} `json:"crawler"`
	Telegram struct {
		ApiId     int32        `json:"apiid"`
		ApiHash   string       `json:"apihash"`
		BotToken  string       `json:"bottoken"`
		DBPath    string       `json:"dbpath"`
		FileStore string       `json:"filestorepath"`
		OTPSeed   string       `json:"otpseed"`
		Client    *tg.Telegram `json:"-"`
	} `json:"telegram"`
	Messages struct {
		tg.TGMessages
		State        string            `json:"state"`
		stateTmpl    *tmpl.Template
		Announce     string            `json:"announce"`
		announceTmpl *tmpl.Template
		Nx           string            `json:"n1x"`
		nxTmpl       *tmpl.Template
		Replacements map[string]string `json:"replacements"`
		Added        string            `json:"added"`
		Updated      string            `json:"updated"`
	} `json:"msg"`
	DBFile string    `json:"dbfile"`
	DB     *Database `json:"-"`
}

var logger = logging.MustGetLogger("observer")

func ReadConfig(path string) (*Observer, error) {
	var config Observer
	confData, err := ioutil.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(confData, &config)
	}
	return &config, err
}

func (cr *Observer) Init() error {
	db := &Database{}
	if err := db.Connect(cr.DBFile); err != nil {
		return err
	}
	cr.DB = db
	if err := cr.initTg(); err != nil {
		return err
	}
	logger.Debug("Initiating meta extractor")
	if len(cr.Crawler.MetaActions) == 0 {
		logger.Warning("extract actions not set")
	} else {
		ex := HTExtractor.New()
		if err := ex.Compile(cr.Crawler.MetaActions); err == nil {
			cr.Crawler.MetaExtractor = ex
		} else {
			return err
		}
	}
	var err error
	if cr.Messages.announceTmpl, err = tmpl.New("announce").Parse(cr.Messages.Announce); err != nil {
		logger.Error(err)
	}
	if cr.Messages.stateTmpl, err = tmpl.New("state").Parse(cr.Messages.State); err != nil {
		logger.Error(err)
	}
	if cr.Messages.nxTmpl, err = tmpl.New("n1000").Parse(cr.Messages.Nx); err != nil {
		logger.Error(err)
	}
	return nil
}

func (cr *Observer) Engage() {
	defer cr.DB.Close()
	defer cr.Telegram.Client.Close()
	var err error
	var nextOffset uint
	if nextOffset, err = cr.DB.GetCrawlOffset(); err == nil {
		go cr.Telegram.Client.HandleUpdates()
		for {
			newNextOffset := nextOffset
			for offsetToCheck := nextOffset; offsetToCheck < nextOffset+cr.Crawler.Threshold; offsetToCheck++ {
				if cr.checkTorrent(offsetToCheck) {
					newNextOffset = offsetToCheck + 1
				}
			}
			if newNextOffset > nextOffset {
				nextOffset = newNextOffset
				if err = cr.DB.UpdateCrawlOffset(nextOffset); err != nil {
					logger.Error(err)
				}
			}
			sleepTime := time.Duration(rand.Intn(int(cr.Crawler.Delay)) + int(cr.Crawler.Delay))
			logger.Debugf("Sleeping %d sec", sleepTime)
			time.Sleep(sleepTime * time.Second)
		}
	} else {
		logger.Fatal(err)
	}
}

func (cr *Observer) checkTorrent(offset uint) bool {
	var res bool
	logger.Debug("Checking offset ", offset)
	fullContext := fmt.Sprintf(cr.Crawler.ContextURL, offset)
	if torrent, err := GetTorrent(cr.Crawler.BaseURL + fullContext); err == nil {
		if torrent != nil {
			logger.Info("New file", torrent.Info.Name)
			newSize := torrent.FullSize()
			logger.Info("New torrent size", newSize)
			if newSize > 0 {
				var torrentId int64
				var isNew bool
				if torrentId, err = cr.DB.GetTorrent(torrent.Info.Name); err != nil {
					logger.Error(err)
				}
				isNew = torrentId == invalidId
				if torrentId, err = cr.DB.AddTorrent(torrent.Info.Name, torrent.Files()); err != nil {
					logger.Error(err)
				}
				cr.notify(torrent, fullContext, torrentId, isNew)
				if offset > 0 && offset%cr.Crawler.Anniversary == 0 {
					cr.nxGet(offset)
				}

				res = true
			} else {
				logger.Error("Zero torrent size, offset", offset)
			}
		} else {
			logger.Debugf("%s not a torrent", fullContext)
		}
	}
	return res
}

func (cr *Observer) notify(torrent *Torrent, context string, torrentId int64, isNew bool) {
	var err error
	var meta map[string]string
	var torrentImageUrl string
	if cr.Crawler.MetaExtractor != nil {
		var rawMeta map[string][]byte
		if rawMeta, err = cr.Crawler.MetaExtractor.ExtractData(cr.Crawler.BaseURL, context);
			err == nil && len(rawMeta) > 0 {
			meta = make(map[string]string, len(rawMeta))
			for k, v := range rawMeta {
				if len(k) > 0 {
					s := strings.TrimSpace(html.UnescapeString(string(v)))
					meta[k] = s
					if k == cr.Crawler.ImageMetaField {
						torrentImageUrl = s
					}
				}
			}
		}
	}
	if err != nil {
		logger.Error(err)
	}
	if len(meta) > 0 {
		if err = cr.DB.AddTorrentMeta(torrentId, meta); err != nil {
			logger.Error(err)
		}
	} else {
		if meta, err = cr.DB.GetTorrentMeta(torrentId); err != nil {
			logger.Error(err)
		}
	}
	var torrentImage []byte
	if torrentImage, err = cr.DB.GetTorrentImage(torrentId); err == nil {
		if len(torrentImage) == 0 {
			err, torrentImage = cr.updateImage(torrentId, torrentImageUrl)
		}
	}
	if err != nil {
		logger.Error(err)
	}
	torrent.Meta = meta
	torrent.Image = torrentImage
	torrent.URL = cr.Crawler.BaseURL + context
	cr.announce(isNew, torrent)
}

func(cr *Observer) updateImage(torrentId int64, imageUrl string) (error,[]byte) {
	var err error
	var torrentImage []byte
	if len(imageUrl) > 0 {
		if strings.Index(imageUrl, cr.Crawler.BaseURL) < 0 {
			imageUrl = cr.Crawler.BaseURL + imageUrl
		}
		if resp, httpErr := http.Get(imageUrl); httpErr == nil && resp != nil && resp.StatusCode < 400 {
			defer resp.Body.Close()
			var img image.Image
			if img, _, err = image.Decode(resp.Body); err == nil && img != nil {
				if cr.Crawler.ImageThumb > 0 {
					img = resize.Thumbnail(cr.Crawler.ImageThumb, cr.Crawler.ImageThumb, img, resize.Bicubic)
				}
				imgBuffer := bytes.Buffer{}
				if err = jpeg.Encode(&imgBuffer, img, nil); err == nil {
					if torrentImage, err = ioutil.ReadAll(&imgBuffer); err == nil {
						err = cr.DB.AddTorrentImage(torrentId, torrentImage)
					}
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
	} else{
		err = errors.New("invalid image url")
	}
	return err, torrentImage
}
