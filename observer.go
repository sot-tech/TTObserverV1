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
	"encoding/json"
	"fmt"
	"github.com/op/go-logging"
	"html"
	"io/ioutil"
	"math/rand"
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
		MetaActions    []HTExtractor.ExtractAction `json:"metaactions"`
		ImageMetaField string                      `json:"imagemetafield"`
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
		stateTmpl    *tmpl.Template    `json:"-"`
		Announce     string            `json:"announce"`
		announceTmpl *tmpl.Template    `json:"-"`
		N1000        string            `json:"n1000"`
		n1000Tmpl    *tmpl.Template    `json:"-"`
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
	if cr.Messages.n1000Tmpl, err = tmpl.New("n1000").Parse(cr.Messages.N1000); err != nil {
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
				if cr.checkTorrent(offsetToCheck){
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
				var existSize uint64
				if existSize, err = cr.DB.GetTorrentSize(torrent.Info.Name); err != nil {
					logger.Error(err)
				}
				cr.notify(torrent, fullContext, existSize == 0)
				if offset%1000 == 0 {
					cr.n1000Get(offset)
				}
				if err = cr.DB.UpdateTorrent(torrent.Info.Name, newSize); err != nil {
					logger.Error(err)
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

func (cr *Observer) notify(torrent *Torrent, context string, isNew bool) {
	var err error
	var meta map[string]string
	var poster []byte
	if cr.Crawler.MetaExtractor != nil {
		var rawMeta map[string][]byte
		if rawMeta, err = cr.Crawler.MetaExtractor.ExtractData(cr.Crawler.BaseURL, context); err == nil && len(rawMeta) > 0 {
			meta = make(map[string]string, len(rawMeta))
			for k, v := range rawMeta {
				if len(k) > 0 {
					if k != cr.Crawler.ImageMetaField {
						meta[k] = strings.TrimSpace(html.UnescapeString(string(v)))
					} else {
						poster = v
					}
				}
			}
		}
	}
	if err != nil{
		logger.Error(err)
	}
	torrent.Meta = meta
	torrent.Poster = poster
	torrent.URL = cr.Crawler.BaseURL + context
	cr.announce(isNew, torrent)
}
