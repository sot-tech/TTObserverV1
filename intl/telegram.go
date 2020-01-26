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

package intl

import (
	"fmt"
	tlg "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/xlzd/gotp"
	"strconv"
	"strings"
	"time"
)

const (
	msgAction       = "${action}"
	msgName         = "${name}"
	msgSize         = "${size}"
	msgUrl          = "${url}"
	msgIndex        = "${index}"
	msgErrorMsg     = "${msg}"
	msgWatch        = "${watch}"
	msgAdmin        = "${admin}"
	msgFileCount    = "${filecount}"
	msgPublisherUrl = "${publisherurl}"

	cmdStart    = "start"
	cmdAttach   = "attach"
	cmdDetach   = "detach"
	cmdState    = "state"
	cmdSetAdmin = "setadmin"
	cmdRmAdmin  = "rmadmin"
	cmdLsAdmins = "lsadmins"
	cmdLsChats  = "lschats"
)

type Messages struct {
	Replacements map[string]string `json:"replacements"`
	Commands     struct {
		Start        string `json:"start"`
		Attach       string `json:"attach"`
		Detach       string `json:"detach"`
		State        string `json:"state"`
		SetAdmin     string `json:"setadmin"`
		RmAdmin      string `json:"rmadmin"`
		Unauthorized string `json:"auth"`
		Unknown      string `json:"unknown"`
	} `json:"cmds"`
	Announce string `json:"announce"`
	N1000    string `json:"n1000"`
	Added    string `json:"added"`
	Updated  string `json:"updated"`
	Error    string `json:"error"`
}

type Telegram struct {
	Messages Messages
	Bot      *tlg.BotAPI
	DB       *Database
	TOTP     *gotp.TOTP
}

func (tg *Telegram) checkOtp(args string) bool {
	return tg.TOTP.Verify(args, int(time.Now().Unix()))
}

func (tg *Telegram) processCommand(msg *tlg.Message) {
	chat := msg.Chat.ID
	resp := tg.Messages.Commands.Unknown
	cmd := msg.Command()
	switch cmd {
	case cmdStart:
		resp = tg.Messages.Commands.Start
	case cmdAttach:
		if err := tg.DB.AddChat(chat); err == nil {
			Logger.Noticef("New chat added %d", chat)
			resp = tg.Messages.Commands.Attach
		} else {
			Logger.Warningf("Attach: %v", err)
			resp = strings.Replace(tg.Messages.Error, msgErrorMsg, err.Error(), -1)
		}
	case cmdDetach:
		if err := tg.DB.DelChat(chat); err == nil {
			Logger.Noticef("Chat deleted %d", chat)
			resp = tg.Messages.Commands.Detach
		} else {
			Logger.Warningf("Attach: %v", err)
			resp = strings.Replace(tg.Messages.Error, msgErrorMsg, err.Error(), -1)
		}
	case cmdState:
		var err error
		var watch, admin bool
		var offset uint
		watch, err = tg.DB.GetChatExist(chat)
		admin, err = tg.DB.GetAdminExist(chat)
		offset, err = tg.DB.GetCrawlOffset()
		if err == nil {
			resp = strings.Replace(tg.Messages.Commands.State, msgWatch, strconv.FormatBool(watch), -1)
			resp = strings.Replace(resp, msgAdmin, strconv.FormatBool(admin), -1)
			resp = strings.Replace(resp, msgIndex, strconv.FormatUint(uint64(offset), 10), -1)
		} else {
			Logger.Warningf("State: %v", err)
			resp = strings.Replace(tg.Messages.Error, msgErrorMsg, err.Error(), -1)
		}
	case cmdSetAdmin:
		if tg.checkOtp(msg.CommandArguments()) {
			if err := tg.DB.AddAdmin(chat); err == nil {
				Logger.Noticef("New admin added %d", chat)
				resp = tg.Messages.Commands.SetAdmin
			} else {
				Logger.Warningf("SetAdmin: %v", err)
				resp = strings.Replace(tg.Messages.Error, msgErrorMsg, err.Error(), -1)
			}
		} else {
			Logger.Infof("SetAdmin unauthorized %d", chat)
			resp = tg.Messages.Commands.Unauthorized
		}
	case cmdRmAdmin:
		if isAdmin, err := tg.DB.GetAdminExist(chat); isAdmin {
			if err := tg.DB.DelAdmin(chat); err == nil {
				Logger.Noticef("Admin deleted %d", chat)
				resp = tg.Messages.Commands.RmAdmin
			} else {
				Logger.Warningf("RmAdmin: %v", err)
				resp = strings.Replace(tg.Messages.Error, msgErrorMsg, err.Error(), -1)
			}
		} else {
			if err == nil {
				Logger.Infof("RmAdmin unauthorized %d", chat)
				resp = tg.Messages.Commands.Unauthorized
			} else {
				Logger.Warningf("RmAdmin: %v", err)
				resp = strings.Replace(tg.Messages.Error, msgErrorMsg, err.Error(), -1)
			}
		}
	case cmdLsAdmins:
		if isAdmin, err := tg.DB.GetAdminExist(chat); isAdmin {
			if admins, err := tg.DB.GetAdmins(); err == nil {
				Logger.Noticef("LsAdmins called %d", chat)
				sb := strings.Builder{}
				for _, id := range admins{
					sb.WriteString(fmt.Sprintf("%d: %s\n", id, tg.GetChatName(id)))
				}
				resp = sb.String()
			} else {
				Logger.Warningf("LsAdmins: %v", err)
				resp = strings.Replace(tg.Messages.Error, msgErrorMsg, err.Error(), -1)
			}
		} else {
			if err == nil {
				Logger.Infof("LsAdmins unauthorized %d", chat)
				resp = tg.Messages.Commands.Unauthorized
			} else {
				Logger.Warningf("LsAdmins: %v", err)
				resp = strings.Replace(tg.Messages.Error, msgErrorMsg, err.Error(), -1)
			}
		}
	case cmdLsChats:
		if isAdmin, err := tg.DB.GetAdminExist(chat); isAdmin {
			if chats, err := tg.DB.GetChats(); err == nil {
				Logger.Noticef("LsChats called %d", chat)
				sb := strings.Builder{}
				for _, id := range chats{
					sb.WriteString(fmt.Sprintf("%d: %s\n", id, tg.GetChatName(id)))
				}
				resp = sb.String()
			} else {
				Logger.Warningf("LsChats: %v", err)
				resp = strings.Replace(tg.Messages.Error, msgErrorMsg, err.Error(), -1)
			}
		} else {
			if err == nil {
				Logger.Infof("LsChats unauthorized %d", chat)
				resp = tg.Messages.Commands.Unauthorized
			} else {
				Logger.Warningf("LsChats: %v", err)
				resp = strings.Replace(tg.Messages.Error, msgErrorMsg, err.Error(), -1)
			}
		}
	}
	tg.sendMsg(resp, nil, []int64{chat}, false)
}

func (tg *Telegram) HandleUpdates() {
	offset, err := tg.DB.GetTgOffset()
	if err != nil {
		Logger.Error(err)
	}
	updateConfig := tlg.NewUpdate(offset)
	updateConfig.Timeout = 60
	if updateChannel, err := tg.Bot.GetUpdatesChan(updateConfig); err == nil {
		for up := range updateChannel {
			Logger.Noticef("Got new update: %v", up)
			var msg *tlg.Message
			if up.Message != nil && up.Message.IsCommand() {
				msg = up.Message
			} else if up.ChannelPost != nil && up.ChannelPost.IsCommand() {
				msg = up.ChannelPost
			}
			if msg != nil {
				go tg.processCommand(msg)
			}
			if up.UpdateID > offset {
				offset = up.UpdateID
				if err = tg.DB.UpdateTgOffset(offset); err != nil {
					Logger.Error(err)
				}
			}
		}
	} else {
		Logger.Errorf("Unable to get telegram update channel, commands disabled: %v", err)
	}
}

func (tg *Telegram) sendMsg(msgText string, msgPhoto []byte, chats []int64, formatted bool) {
	if msgText != "" && chats != nil && len(chats) > 0 {
		Logger.Debugf("Sending message %s to %v", msgText, chats)
		var photoId string
		for _, chat := range chats {
			var msg tlg.Chattable
			if msgPhoto != nil {
				var photoMsg tlg.PhotoConfig
				if photoId == "" {
					photoMsg = tlg.NewPhotoUpload(chat, tlg.FileBytes{Bytes: msgPhoto})
				} else {
					photoMsg = tlg.NewPhotoShare(chat, photoId)
				}
				photoMsg.Caption = msgText
				if formatted {
					photoMsg.ParseMode = "Markdown"
				}
				msg = photoMsg
			} else {
				msgText := tlg.NewMessage(chat, msgText)
				if formatted {
					msgText.ParseMode = "Markdown"
				}
				msg = msgText
			}
			if sentMsg, err := tg.Bot.Send(msg); err == nil {
				Logger.Debugf("Message to %d has been sent", chat)
				if photoId == "" && sentMsg.Photo != nil && len(*sentMsg.Photo) > 0 {
					photoId = (*sentMsg.Photo)[0].FileID
				}
			} else {
				Logger.Error(err)
			}
		}
	}
}

func (tg *Telegram) SendMsgToAdmins(msg string, photo []byte) {
	var chats []int64
	var err error
	if chats, err = tg.DB.GetAdmins(); err != nil {
		Logger.Error(err)
	}
	tg.sendMsg(msg, photo, chats, false)
}

func (tg *Telegram) SendMsgToMobs(msg string, photo []byte) {
	var chats []int64
	var err error
	if chats, err = tg.DB.GetChats(); err != nil {
		Logger.Error(err)
	}
	tg.sendMsg(msg, photo, chats, true)
}

func (tg *Telegram) announce(action string, torrent *Torrent) {
	if tg.Messages.Announce == "" {
		Logger.Warning("Announce message not set")
	} else {
		Logger.Debugf("Announcing %s for %s", action, torrent.Info.Name)
		msg := tg.Messages.Announce
		name := torrent.PrettyName
		if tg.Messages.Replacements != nil {
			for k, v := range tg.Messages.Replacements {
				name = strings.Replace(name, k, v, -1)
			}
		}
		msg = strings.Replace(msg, msgAction, action, -1)
		msg = strings.Replace(msg, msgName, name, -1)
		msg = strings.Replace(msg, msgSize, torrent.StringSize(), -1)
		msg = strings.Replace(msg, msgUrl, torrent.URL, -1)
		msg = strings.Replace(msg, msgFileCount, strconv.FormatUint(torrent.FileCount(), 10), -1)
		msg = strings.Replace(msg, msgPublisherUrl, torrent.PublisherUrl, -1)
		tg.SendMsgToMobs(msg, torrent.Poster)
	}
}

func (tg *Telegram) AnnounceNew(torrent *Torrent) {
	tg.announce(tg.Messages.Added, torrent)
}

func (tg *Telegram) AnnounceUpdate(torrent *Torrent) {
	tg.announce(tg.Messages.Updated, torrent)
}

func (tg *Telegram) N1000Get(offset uint) {
	if tg.Messages.N1000 == "" {
		Logger.Warning("N1000 message not set")
	} else {
		Logger.Debugf("Notifying %d GET", offset)
		tg.SendMsgToMobs(strings.Replace(tg.Messages.N1000, msgIndex, strconv.FormatUint(uint64(offset), 10), -1), nil)
	}
}

func (tg *Telegram) UnaiwailNotify(count uint, err string) {
	if tg.Messages.Error == "" {
		Logger.Warning("Error message not set")
	} else {
		Logger.Noticef("Notifying about error: try %d, message %s", count, err)
		msg := strings.Replace(tg.Messages.Error, msgErrorMsg, fmt.Sprintf("%s (try %d)", err, count), -1)
		tg.SendMsgToAdmins(msg, nil)
	}
}

func (tg *Telegram) GetChatName(chatId int64) string {
	name := "Unknown"
	if chat, err := tg.Bot.GetChat(tlg.ChatConfig{
		ChatID: chatId,
	}); err == nil {
		name = chat.UserName
	} else {
		Logger.Warning(err)
	}
	return name
}

func (tg *Telegram) Connect(token, otpSeed string, tries int) error {
	var err error
	tg.TOTP = gotp.NewDefaultTOTP(otpSeed)
	for try := 0; try < tries || tries < 0; try++ {
		if tg.Bot, err = tlg.NewBotAPI(token); err == nil {
			tg.Bot.Debug = false
			Logger.Infof("Authorized on account %s", tg.Bot.Self.UserName)
			break
		} else {
			Logger.Errorf("Unable to connect to telegram, try %d of %d: %s\n", try, tries, err)
			time.Sleep(10 * time.Second)
		}
	}
	return err
}
