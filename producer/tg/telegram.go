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

package tg

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	tmpl "text/template"

	"github.com/op/go-logging"
	mt "sot-te.ch/MTHelper"

	"sot-te.ch/TTObserverV1/producer"
	s "sot-te.ch/TTObserverV1/shared"
)

const (
	msgWatch = "watch"
	msgAdmin = "admin"

	cmdLsAdmins     = "/lsadmins"
	cmdLsChats      = "/lschats"
	cmdUpdatePoster = "/uploadposter"
)

var (
	logger      = logging.MustGetLogger("tg")
	errNotFound = errors.New("not found")
)

func init() {
	producer.RegisterFactory("telegram", new(Notifier))
}

type messageTemplates struct {
	state           *tmpl.Template
	announce        *tmpl.Template
	nx              *tmpl.Template
	singleIndex     *tmpl.Template
	multipleIndexes *tmpl.Template
}

type Notifier struct {
	ApiId     int32  `json:"apiid"`
	ApiHash   string `json:"apihash"`
	BotToken  string `json:"bottoken"`
	DBPath    string `json:"dbpath"`
	FileStore string `json:"filestorepath"`
	OTPSeed   string `json:"otpseed,omitempty"`
	Messages  struct {
		mt.TGMessages
		State           string            `json:"state"`
		Announce        string            `json:"announce,omitempty"`
		Nx              string            `json:"n1x,omitempty"`
		Replacements    map[string]string `json:"replacements"`
		Added           string            `json:"added,omitempty"`
		Updated         string            `json:"updated,omitempty"`
		SingleIndex     string            `json:"singleindex,omitempty"`
		MultipleIndexes string            `json:"multipleindexes,omitempty"`
	} `json:"msg"`
	messages        *messageTemplates
	db              s.Database
	client          *mt.Telegram
	errUnauthorized error
}

func (tg *Notifier) getChats(chat int64, admins bool) error {
	var err error
	var resp string
	var isAdmin bool
	if isAdmin, err = tg.db.GetAdminExist(chat); isAdmin {
		var chats []int64
		if admins {
			chats, err = tg.db.GetAdmins()
		} else {
			chats, err = tg.db.GetChats()
		}
		if err == nil {
			logger.Notice("LsChats called", chat)
			sb := strings.Builder{}
			for _, id := range chats {
				var title []string
				if title, err = tg.client.GetChatTitle(id); err != nil {
					title = append(title, err.Error())
					err = nil
				}
				sb.WriteString(fmt.Sprintf("%d: %s\n", id, title))
			}
			resp = sb.String()
		} else {
			logger.Warningf("LsChats: %v", err)
		}
	} else {
		if err == nil {
			logger.Infof("LsChats unauthorized %d", chat)
			err = tg.errUnauthorized
		} else {
			logger.Warningf("LsChats: %v", err)
		}
	}
	if err == nil {
		tg.client.SendMsg(resp, []int64{chat}, false)
	}
	return err
}

func (tg *Notifier) getState(chat int64) (string, error) {
	var err error
	var isMob, isAdmin bool
	var index uint
	if isMob, err = tg.db.GetChatExist(chat); err != nil {
		return "", err
	}
	if isAdmin, err = tg.db.GetAdminExist(chat); err != nil {
		return "", err
	}
	if index, err = tg.db.GetCrawlOffset(); err != nil {
		return "", err
	}
	return producer.FormatMessage(tg.messages.state, map[string]any{
		msgWatch:          isMob,
		msgAdmin:          isAdmin,
		producer.MsgIndex: index,
	})
}

func (tg *Notifier) uploadPoster(chat int64, args []string) error {
	var err error
	var isAdmin bool
	if isAdmin, err = tg.db.GetAdminExist(chat); isAdmin {
		var torrentId int64
		if len(args) < 2 || len(args[0]) == 0 || len(args[1]) == 0 {
			err = s.ErrRequiredParameters
		} else if torrentId, err = strconv.ParseInt(args[0], 10, 64); err == nil {
			var exist bool
			if exist, err = tg.db.CheckTorrent(torrentId); err == nil {
				if exist {
					var torrentPoster []byte
					if torrentPoster, err = s.GetTorrentPoster(args[1], 0); err == nil {
						if err = tg.db.AddTorrentImage(torrentId, torrentPoster); err == nil {
							tg.client.SendMsg(tg.Messages.Added, []int64{chat}, false)
						}
					}
				} else {
					err = errNotFound
				}
			}
		}
	} else {
		if err == nil {
			logger.Infof("UploadPoster unauthorized %d", chat)
			err = tg.errUnauthorized
		} else {
			logger.Warningf("UploadPoster: %v", err)
		}
	}
	return err
}

func (tg *Notifier) init() error {
	var err error
	tg.client = mt.New(tg.ApiId, tg.ApiHash, tg.DBPath, tg.FileStore, tg.OTPSeed)
	tg.messages = new(messageTemplates)
	tg.client.Messages = tg.Messages.TGMessages
	tg.client.BackendFunctions = mt.TGBackendFunction{
		ChatExist:  tg.db.GetChatExist,
		ChatAdd:    tg.db.AddChat,
		ChatRm:     tg.db.DelChat,
		AdminExist: tg.db.GetAdminExist,
		AdminAdd:   tg.db.AddAdmin,
		AdminRm:    tg.db.DelAdmin,
		State:      tg.getState,
	}
	if err = tg.client.LoginAsBot(tg.BotToken, mt.MtLogWarning); err == nil {
		var subErr error
		if subErr = tg.client.AddCommand(cmdLsChats, func(chat int64, _ string, _ []string) error {
			return tg.getChats(chat, false)
		}); subErr != nil {
			logger.Error(subErr)
		}
		if subErr = tg.client.AddCommand(cmdLsAdmins, func(chat int64, _ string, _ []string) error {
			return tg.getChats(chat, true)
		}); subErr != nil {
			logger.Error(subErr)
		}
		if subErr = tg.client.AddCommand(cmdUpdatePoster, func(chat int64, _ string, args []string) error {
			return tg.uploadPoster(chat, args)
		}); subErr != nil {
			logger.Error(subErr)
		}
		if tg.messages.announce, subErr = tmpl.New("announce").Parse(tg.Messages.Announce); subErr != nil {
			logger.Error(subErr)
		}
		if tg.messages.state, subErr = tmpl.New("state").Parse(tg.Messages.State); subErr != nil {
			logger.Error(subErr)
		}
		if tg.messages.nx, subErr = tmpl.New("n1000").Parse(tg.Messages.Nx); subErr != nil {
			logger.Error(subErr)
		}
		if tg.messages.singleIndex, subErr = tmpl.New("singleIndex").Parse(tg.Messages.SingleIndex); subErr != nil {
			logger.Error(subErr)
		}
		if tg.messages.multipleIndexes, subErr = tmpl.New("multipleIndexes").Parse(tg.Messages.MultipleIndexes); subErr != nil {
			logger.Error(subErr)
		}
		tg.errUnauthorized = errors.New(tg.Messages.Unauthorized)
	}
	return err
}

func (tg *Notifier) sendMsgToMobs(msg string, photo []byte) {
	var chats []int64
	var err error
	if chats, err = tg.db.GetChats(); err != nil {
		logger.Error(err)
	}
	photoParams := mt.MediaParams{}
	if len(photo) > 0 {
		ext := "*."
		if exts := strings.Split(http.DetectContentType(photo), "/"); len(exts) > 1 {
			ext += exts[1]
		}
		var tmpFile *os.File
		if tmpFile, err = os.CreateTemp("", ext); err == nil {
			if _, err = tmpFile.Write(photo); err == nil {
				_ = tmpFile.Sync()
				photoParams.Path = tmpFile.Name()
			}
			if err = tmpFile.Close(); err != nil {
				logger.Error(err)
			}
		}
	}
	tg.client.SendPhoto(photoParams, msg, chats, true)
	if len(photoParams.Path) > 0 {
		if err = os.Remove(photoParams.Path); err != nil {
			logger.Error(err)
		}
	}
}

func (*Notifier) New(configPath string, db s.Database) (producer.Producer, error) {
	var err error
	n := &Notifier{db: db}
	var confBytes []byte
	if confBytes, err = os.ReadFile(filepath.Clean(configPath)); err == nil {
		if err = json.Unmarshal(confBytes, n); err == nil {
			if err = n.init(); err == nil {
				go n.client.HandleUpdates()
			}
		}
	}
	return n, err
}

func (tg *Notifier) Send(isNew bool, torrent *s.TorrentInfo) {
	if tg.Messages.Announce == "" {
		logger.Warning("Announce message not set")
	} else {
		action := tg.Messages.Updated
		if isNew {
			action = tg.Messages.Added
		}
		logger.Debugf("Announcing %s for %s", action, torrent.Name)
		name := torrent.Name
		if len(tg.Messages.Replacements) > 0 {
			for k, v := range tg.Messages.Replacements {
				name = strings.Replace(name, k, v, -1)
			}
		}
		newIndexes, err := producer.FormatIndexesMessage(producer.GetNewFilesIndexes(torrent.Files), tg.messages.singleIndex,
			tg.messages.multipleIndexes, producer.MsgNewIndexes)
		if err != nil {
			logger.Error(err)
		}
		if msg, err := producer.FormatMessage(tg.messages.announce, map[string]any{
			producer.MsgAction:     action,
			producer.MsgName:       name,
			producer.MsgSize:       producer.FormatFileSize(torrent.Length),
			producer.MsgUrl:        torrent.URL,
			producer.MsgFileCount:  len(torrent.Files),
			producer.MsgMeta:       torrent.Meta,
			producer.MsgNewIndexes: newIndexes,
		}); err == nil {
			tg.sendMsgToMobs(msg, torrent.Image)
		} else {
			logger.Error(err)
		}
	}
}

func (tg *Notifier) SendNxGet(offset uint) {
	if len(tg.Messages.Nx) == 0 {
		logger.Warning("Nx message not set")
	} else {
		logger.Debugf("Notifying %d GET", offset)
		if msg, err := producer.FormatMessage(tg.messages.nx, map[string]any{
			producer.MsgIndex: offset,
		}); err == nil {
			tg.sendMsgToMobs(msg, nil)
		} else {
			logger.Error(err)
		}
	}
}

func (tg *Notifier) Close() {
	tg.client.Close()
}
