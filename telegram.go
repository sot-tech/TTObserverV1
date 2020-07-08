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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	tg "sot-te.ch/MTHelper"
	"strings"
	"text/template"
)

const (
	msgAction    = "action"
	msgName      = "name"
	msgSize      = "size"
	msgUrl       = "url"
	msgIndex     = "index"
	msgWatch     = "watch"
	msgAdmin     = "admin"
	msgFileCount = "filecount"
	msgMeta      = "meta"

	cmdLsAdmins     = "/lsadmins"
	cmdLsChats      = "/lschats"
	cmdLsReleases   = "/lsreleases"
	cmdUpdatePoster = "/uploadposter"
)

func formatMessage(tmpl *template.Template, values map[string]interface{}) (string, error) {
	var err error
	var res string
	if tmpl != nil {
		buf := bytes.Buffer{}
		if err = tmpl.Execute(&buf, values); err == nil {
			res = buf.String()
		}
	} else {
		err = errors.New("template not inited")
	}
	return res, err
}

func (cr *Observer) getChats(chat int64, admins bool) error {
	var err error
	var resp string
	var isAdmin bool
	if isAdmin, err = cr.DB.GetAdminExist(chat); isAdmin {
		var chats []int64
		if admins {
			chats, err = cr.DB.GetAdmins()
		} else {
			chats, err = cr.DB.GetChats()
		}
		if err == nil {
			logger.Notice("LsChats called", chat)
			sb := strings.Builder{}
			for _, id := range chats {
				var title string
				if title, err = cr.Telegram.Client.GetChatTitle(id); err != nil {
					title = err.Error()
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
			err = errors.New(cr.Messages.Unauthorized)
		} else {
			logger.Warningf("LsChats: %v", err)
		}
	}
	if err == nil {
		cr.Telegram.Client.SendMsg(resp, []int64{chat}, false)
	}
	return err
}

func (cr *Observer) getState(chat int64) (string, error) {
	var err error
	var isMob, isAdmin bool
	var index uint
	if isMob, err = cr.DB.GetChatExist(chat); err != nil {
		return "", err
	}
	if isAdmin, err = cr.DB.GetAdminExist(chat); err != nil {
		return "", err
	}
	if index, err = cr.DB.GetCrawlOffset(); err != nil {
		return "", err
	}
	return formatMessage(cr.Messages.stateTmpl, map[string]interface{}{
		msgWatch: isMob,
		msgAdmin: isAdmin,
		msgIndex: index,
	})
}

func (cr *Observer) getReleases(chat int64, args string) error {
	var err error
	var isAdmin bool
	if isAdmin, err = cr.DB.GetAdminExist(chat); isAdmin {
		var values strings.Builder
		var torrents []DBTorrent
		if torrents, err = cr.DB.GetTorrents(args); err == nil {
			if len(torrents) > 0 {
				for _, torrent := range torrents {
					values.WriteString(torrent.String())
					values.WriteRune('\n')
				}
				cr.Telegram.Client.SendMsg(values.String(), []int64{chat}, false)
			} else {
				err = errors.New("not found")
			}
		}
	} else {
		if err == nil {
			logger.Infof("LsReleases unauthorized %d", chat)
			err = errors.New(cr.Messages.Unauthorized)
		} else {
			logger.Warningf("LsReleases: %v", err)
		}
	}
	return err
}

func (cr *Observer) uploadPoster(chat int64, args string) error {
	var err error
	var isAdmin bool
	if isAdmin, err = cr.DB.GetAdminExist(chat); isAdmin {
		var torrentId int64
		var posterUrl string
		if _, err = fmt.Sscanf(args,"%d %s", &torrentId, &posterUrl); err == nil {
			var exist bool
			if exist, err = cr.DB.CheckTorrent(torrentId); err == nil{
				if exist {
					if err, _ = cr.updateImage(torrentId, posterUrl); err == nil{
						cr.Telegram.Client.SendMsg(cr.Messages.Added, []int64{chat}, false)
					}
				} else{
					err = errors.New("not found")
				}
			}
		}
	} else {
		if err == nil {
			logger.Infof("LsReleases unauthorized %d", chat)
			err = errors.New(cr.Messages.Unauthorized)
		} else {
			logger.Warningf("LsReleases: %v", err)
		}
	}
	return err
}

func (cr *Observer) initTg() error {
	var err error
	telegram := tg.New(cr.Telegram.ApiId, cr.Telegram.ApiHash, cr.Telegram.DBPath, cr.Telegram.FileStore, cr.Telegram.OTPSeed)
	telegram.Messages = cr.Messages.TGMessages
	telegram.BackendFunctions = tg.TGBackendFunction{
		GetOffset:  cr.DB.GetTgOffset,
		SetOffset:  cr.DB.UpdateTgOffset,
		ChatExist:  cr.DB.GetChatExist,
		ChatAdd:    cr.DB.AddChat,
		ChatRm:     cr.DB.DelChat,
		AdminExist: cr.DB.GetAdminExist,
		AdminAdd:   cr.DB.AddAdmin,
		AdminRm:    cr.DB.DelAdmin,
		State:      cr.getState,
	}
	if err = telegram.LoginAsBot(cr.Telegram.BotToken, tg.MtLogWarning); err == nil {
		cr.Telegram.Client = telegram
		_ = cr.Telegram.Client.AddCommand(cmdLsChats, func(chat int64, _, _ string) error {
			return cr.getChats(chat, false)
		})
		_ = cr.Telegram.Client.AddCommand(cmdLsAdmins, func(chat int64, _, _ string) error {
			return cr.getChats(chat, true)
		})
		_ = cr.Telegram.Client.AddCommand(cmdLsReleases, func(chat int64, _, args string) error {
			return cr.getReleases(chat, args)
		})
		_ = cr.Telegram.Client.AddCommand(cmdUpdatePoster, func(chat int64, _, args string) error {
			return cr.uploadPoster(chat, args)
		})
	}
	return err
}

func (cr *Observer) sendMsgToMobs(msg string, photo []byte) {
	var chats []int64
	var err error
	if chats, err = cr.DB.GetChats(); err != nil {
		logger.Error(err)
	}
	photoParams := tg.MediaParams{}
	if len(photo) > 0 {
		ext := "*."
		if exts := strings.Split(http.DetectContentType(photo), "/"); len(exts) > 1 {
			ext += exts[1]
		}
		var tmpFile *os.File
		if tmpFile, err = ioutil.TempFile("", ext); err == nil {
			if _, err = tmpFile.Write(photo); err == nil {
				_ = tmpFile.Sync()
				photoParams.Path = tmpFile.Name()
			}
			if err = tmpFile.Close(); err != nil {
				logger.Error(err)
			}
		}
	}
	cr.Telegram.Client.SendPhoto(photoParams, msg, chats, true)
	if len(photoParams.Path) > 0 {
		if err = os.Remove(photoParams.Path); err != nil {
			logger.Error(err)
		}
	}
}

func (cr *Observer) announce(new bool, torrent *Torrent) {
	if cr.Messages.Announce == "" {
		logger.Warning("Announce message not set")
	} else {
		action := cr.Messages.Updated
		if new {
			action = cr.Messages.Added
		}
		logger.Debugf("Announcing %s for %s", action, torrent.Info.Name)
		name := torrent.Info.Name
		if len(cr.Messages.Replacements) > 0 {
			for k, v := range cr.Messages.Replacements {
				name = strings.Replace(name, k, v, -1)
			}
		}
		if msg, err := formatMessage(cr.Messages.announceTmpl, map[string]interface{}{
			msgAction:    action,
			msgName:      name,
			msgSize:      torrent.StringSize(),
			msgUrl:       torrent.URL,
			msgFileCount: torrent.FileCount(),
			msgMeta:      torrent.Meta,
		}); err == nil {
			cr.sendMsgToMobs(msg, torrent.Image)
		} else {
			logger.Error(err)
		}
	}
}

func (cr *Observer) nxGet(offset uint) {
	if cr.Messages.Nx == "" {
		logger.Warning("Nx message not set")
	} else {
		logger.Debugf("Notifying %d GET", offset)
		if msg, err := formatMessage(cr.Messages.nxTmpl, map[string]interface{}{
			msgIndex: offset,
		}); err == nil {
			cr.sendMsgToMobs(msg, nil)
		} else {
			logger.Error(err)
		}
	}
}
