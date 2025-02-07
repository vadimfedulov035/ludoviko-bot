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
	CID int64
}

func NewTgInfo(bot *tg.BotAPI, msg *tg.Message, cid int64) *TgInfo {
	return &TgInfo{
		Bot: bot,
		Msg: msg,
		CID: cid,
	}
}

// transforms general config to the custome one
func GetCustomConfig(generalConfig string, order string) string {
	configPostfix := strings.Replace(order, "/", "_", 1)
	customConfig := fmt.Sprintf(generalConfig, configPostfix)
	return customConfig
}

// gets message text if any, sets ok flag
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

// substitutes bot's user name mention to bot's first name addressing in text
func HumanizeBotMention(text string, self *tg.User) string {
	botName, botFirstName := self.UserName, self.FirstName
	text = strings.Replace(text, "@"+botName, botFirstName+",", -1)
	return text
}

// gets name if any; or sets "anonymous"
func GetUserName(msg *tg.Message, capitalize bool) string {
	var userName string

	// bot -> first name, user -> user name
	if msg.From.IsBot {
		userName = msg.From.FirstName
	} else {
		userName = msg.From.UserName
	}

	// hidden user -> anonymous
	if userName == "" {
		userName = "anonymous"
	}

	// capitalize if asked
	if capitalize {
		caser := cases.Title(language.English)
		userName = caser.String(userName)
	}

	return userName
}

// gets chat title if any; or sets "User's chat"
func GetChatTitle(msg *tg.Message) string {
	chatName := msg.Chat.Title
	if chatName == "" {
		chatName = fmt.Sprintf("%s's chat", GetUserName(msg, true))
	}
	return chatName
}

// gets Chat ID or User ID as private Chat ID
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
	text, _ := GetMsgText(msg)

	// get replied message if any for reply check
	replied := msg.ReplyToMessage
	var repliedID int64 = 0
	if replied != nil {
		repliedID = replied.From.ID
	}
	// get order for order check
	order := getOrder(text, orders)
	// get uncaptialized user name for correct admin check
	userName := GetUserName(msg, false)

	// chat status
	isPublic := chat.IsGroup() || chat.IsSuperGroup()
	isPrivate := chat.IsPrivate()

	// bot reply conditions
	isReplied := repliedID == botID
	isMentioned := strings.Contains(text, botName)
	isOrdered := order != ""
	isAdmin := slices.Contains(admins, userName)

	// bot chat reply conditions
	isAskedPublicly := isPublic && (isReplied || isMentioned || isOrdered)
	isAskedPrivately := isPrivate && isAdmin

	// bot ask status
	isAsked := isAskedPublicly || isAskedPrivately

	return isAsked, order
}

// tries to reply the message with text; sends separate message on failure
func Reply(tgInfo *TgInfo, text string) *tg.Message {
	bot, msg := tgInfo.Bot, tgInfo.Msg

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
func Typing(ctx context.Context, tgInfo *TgInfo) {
	bot, cid := tgInfo.Bot, tgInfo.CID
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			actConf := tg.NewChatAction(cid, "typing")
			_, err := bot.Request(actConf)
			if err != nil {
				log.Printf("[Telegram] Action: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
