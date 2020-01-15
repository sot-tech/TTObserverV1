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
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"time"
)

type Telegram struct {
	Token string
	Bot   *tgbotapi.BotAPI
	DB    *Database
}

func (tg *Telegram) HandleCommands() {
	offset, err := tg.DB.GetTgOffset()
	if err != nil{
		logger.Error(err)
	}
	updateConfig := tgbotapi.NewUpdate(offset)
	updateConfig.Timeout = 60
	if updateChannel, err := tg.Bot.GetUpdatesChan(updateConfig); err == nil {
		for up := range updateChannel {
			logger.Notice(up)
			if up.UpdateID > offset{
				offset = up.UpdateID
				if err = tg.DB.UpdateTgOffset(offset); err != nil{
					logger.Error(err)
				}
			}
		}
	} else {
		logger.Errorf("Unable to get telegram update channel, commands disabled: %v", err)
	}
}

func (tg *Telegram) SendMsg(msg string, chat int64) {
	var chats []int64
	var err error
	if chat == 0 {
		if chats, err = tg.DB.GetChats(); err != nil{
			logger.Error(err)
		}
	} else {
		chats = append(chats, chat)
	}
	for _, chat := range chats {
		msg := tgbotapi.NewMessage(chat, msg)
		if _, err = tg.Bot.Send(msg); err != nil {
			logger.Error(err)
		}
	}
}

func (tg *Telegram) Engage(tries int) error {
	var err error
	for try := 0; try < tries || tries < 0; try++ {
		if tg.Bot, err = tgbotapi.NewBotAPI(tg.Token); err == nil {
			tg.Bot.Debug = false
			logger.Noticef("Authorized on account %s", tg.Bot.Self.UserName)
			go tg.HandleCommands()
			break
		} else {
			logger.Error("Unable to connect to telegram, try %d of %d: %s\n", try, tries, err)
			time.Sleep(10 * time.Second)
		}
	}
	return err
}
