package intl

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/op/go-logging"
	"time"
)


var logger = logging.MustGetLogger("telegram")

type Telegram struct {
	Token string
	Bot *tgbotapi.BotAPI
	DB *Database
}

func (tg *Telegram) HandleCommands() {
	updateConfig := tgbotapi.NewUpdate(tg.DB.GetTgOffset())
	updateConfig.Timeout = 60
	if updateChannel, err := tg.Bot.GetUpdatesChan(updateConfig); err == nil {
		for up := range updateChannel {
			logger.Notice(up)
		}
	} else {
		logger.Errorf("Unable to get telegram update channel, commands disabled: %v", err)
	}
}


func (tg *Telegram) SendMsg(msg string, chat int64) {
	var chats []int64
	if chat == 0 {
		chats = tg.DB.GetChats()
	} else {
		chats = append(chats, chat)
	}
	for _, chat := range chats {
		msg := tgbotapi.NewMessage(chat, msg)
		if _, err := tg.Bot.Send(msg); err != nil {
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
