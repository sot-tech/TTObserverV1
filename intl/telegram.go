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
	tlg "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/xlzd/gotp"
	"strings"
	"time"
)

const (
	msgAnnounceAction = "${action}"
	msgAnnounceName   = "${name}"
	msgAnnounceSize   = "${size}"
	msgAnnounceUrl    = "${url}"
	msgGetIndex       = "${index}"
	msgErrorMsg       = "${msg}"
	msgErrorTry       = "${try}"

	cmdStart    = "/start"
	cmdAttach   = "/attach"
	cmdDetach   = "/detach"
	cmdSetAdmin = "/setadmin"
	cmdRmAdmin  = "/rmadmin"
)

type Messages struct {
	Replacements map[string]string `json:"replacements"`
	Commands     struct {
		Start        string `json:"start"`
		Attach       string `json:"attach"`
		Detach       string `json:"detach"`
		SetAdmin     string `json:"setadmin"`
		RmAdmin      string `json:"rmadmin"`
		Unauthorized string `json:"auth"`
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

func (tg *Telegram) processCommand(msg *tlg.Message) {
	msg.Chat.ID
	switch msg.Command(){
	case cmdStart:

	}
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
			} else if up.ChannelPost != nil && up.Message.IsCommand() {
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

func (tg *Telegram) sendMsg(msg string, chats []int64) {
	if msg != "" && len(chats) > 0 {
		var err error
		Logger.Debugf("Sending message %s to %v", msg, chats)
		for _, chat := range chats {
			msg := tlg.NewMessage(chat, msg)
			msg.ParseMode = "Markdown"
			if _, err = tg.Bot.Send(msg); err == nil {
				Logger.Debugf("Message to %d has been sent", chat)
			} else {
				Logger.Error(err)
			}
		}
	}
}

func (tg *Telegram) SendMsgToAdmins(msg string) {
	var chats []int64
	var err error
	if chats, err = tg.DB.GetAdmins(); err != nil {
		Logger.Error(err)
	}
	tg.sendMsg(msg, chats)
}

func (tg *Telegram) SendMsgToMobs(msg string) {
	var chats []int64
	var err error
	if chats, err = tg.DB.GetChats(); err != nil {
		Logger.Error(err)
	}
	tg.sendMsg(msg, chats)
}

func (tg *Telegram) announce(action string, torrent *Torrent) {
	if tg.Messages.Announce == "" {
		Logger.Warning("Announce message not set")
	} else {
		Logger.Debugf("Announcing %s for %s", action, torrent.Info.Name)
		msg := tg.Messages.Announce
		name := torrent.Info.Name
		if tg.Messages.Replacements != nil {
			for k, v := range tg.Messages.Replacements {
				name = strings.Replace(name, k, v, -1)
			}
		}
		msgReplacements := map[string]string{
			msgAnnounceAction: action,
			msgAnnounceName:   name,
			msgAnnounceSize:   torrent.StringSize(),
			msgAnnounceUrl:    torrent.URL,
		}

		for k, v := range msgReplacements {
			msg = strings.Replace(msg, k, v, -1)
		}

		tg.SendMsgToMobs(msg)
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
		tg.SendMsgToMobs(strings.Replace(tg.Messages.N1000, msgGetIndex, string(offset), -1))
	}
}

func (tg *Telegram) UnaiwailNotify(count uint, err string) {
	if tg.Messages.Error == "" {
		Logger.Warning("Error message not set")
	} else {
		Logger.Noticef("Notifying about error: try %d, message %s", count, err)
		msg := strings.Replace(tg.Messages.Error, msgErrorMsg, err, -1)
		msg = strings.Replace(err, msgErrorTry, string(count), -1)
		tg.SendMsgToAdmins(msg)
	}
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
