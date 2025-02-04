package messaging

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TgInfo struct {
	Bot *tg.BotAPI
	Msg *tg.Message
}

func NewTgInfo(bot *tg.BotAPI, msg *tg.Message) *TgInfo {
	return &TgInfo{
		Bot: bot,
		Msg: msg,
	}
}

// transforms general config to custome one
func GetCustomConfig(generalConfig string, order string) string {
	configPostfix := strings.Replace(order, "/", "_", 1)
	customConfig := fmt.Sprintf(generalConfig, configPostfix)
	return customConfig
}

// gets message text if any; returns ok flag
func GetMsgText(msg *tg.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	if msg.Text != "" {
		return msg.Text, true
	}
	if msg.Caption != "" {
		return msg.Caption, true
	}
	return "", false
}

// gets user name if any; or "Anonymous"
func GetUserName(msg *tg.Message) string {
	var userName string

	// bot -> first name
	// user -> capitalized username
	if msg.From.IsBot {
		userName = msg.From.FirstName
	} else {
		caser := cases.Title(language.English)
		userName = caser.String(msg.From.UserName)
	}
	// anon -> Anonymous
	if userName == "" {
		userName = "Anonymous"
	}
	return userName
}

// gets chat title if any; or "User's chat"
func GetChatTitle(msg *tg.Message) string {
	chatName := msg.Chat.Title
	if chatName == "" {
		chatName = fmt.Sprintf("%s's chat", GetUserName(msg))
	}
	return chatName
}

// gets chat ID if any; or gets user ID
func GetCID(msg *tg.Message) int64 {
	var cid int64
	if msg.Chat != nil {
		cid = msg.Chat.ID
	} else {
		cid = msg.From.ID
	}
	return cid
}

// gets order if any
func getOrder(text string, orders []string) string {
	order := ""
	for _, order := range orders {
		if strings.Contains(text, order) {
			return order
		}
	}
	return order
}

// checks if the bot is asked; gets order if any
func Inspect(tgInfo *TgInfo, admins []string, orders []string) (bool, string) {
	bot, msg := tgInfo.Bot, tgInfo.Msg

	self := bot.Self
	botName, botID := self.UserName, self.ID

	chat := msg.Chat
	userName := GetUserName(msg)
	text, _ := GetMsgText(msg)

	replied := msg.ReplyToMessage
	var repliedID int64 = 0
	if replied != nil {
		repliedID = replied.From.ID
	}

	order := getOrder(text, orders)

	isPublic := chat.IsGroup() || chat.IsSuperGroup()
	isPrivate := chat.IsPrivate()

	isReplied := repliedID == botID
	isMentioned := strings.Contains(text, botName)
	isOrdered := order != ""
	isAdmin := slices.Contains(admins, userName)

	isAskedPublicly := isPublic && (isReplied || isMentioned || isOrdered)
	isAskedPrivately := isPrivate && isAdmin

	isAsked := isAskedPublicly || isAskedPrivately

	return isAsked, order
}

// tries to reply the message with text; sends separate message on failure
func Reply(bot *tg.BotAPI, msg *tg.Message, text string) *tg.Message {

	msgConf := tg.NewMessage(msg.Chat.ID, text)
	msgConf.ReplyToMessageID = msg.MessageID

	response, err := bot.Send(msgConf)
	if err != nil {
		msgConf.ReplyToMessageID = 0
		response, err = bot.Send(msgConf)
	}
	if err != nil {
		log.Printf("[Telegram] Replying: %v", err)
	}

	return &response
}

// types every 3 seconds until context done
func Typing(ctx context.Context, bot *tg.BotAPI, id int64) {
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			actConf := tg.NewChatAction(id, "typing")
			_, err := bot.Request(actConf)
			if err != nil {
				log.Printf("[Telegram] Action: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
