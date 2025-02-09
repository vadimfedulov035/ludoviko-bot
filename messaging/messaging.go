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
	Sender string
	Text string
}

func NewTgInfo(bot *tg.BotAPI, msg *tg.Message) *TgInfo {
	if msg == nil {
		return nil
	}
	return &TgInfo{
		Bot: bot,
		Msg: msg,
		Sender: getName(msg, true),
		Text: getText(msg),
	}
}

type ChatInfo struct {
	TgInfo
	Lines []string
	CID int64
	ChatTitle string
	Config string
	Order string
	MemLim int
}

func NewChatInfo(t *TgInfo, conf string, orders []string, lim int) *ChatInfo {
	bot, msg, sender, text := t.Bot, t.Msg, t.Sender, t.Text
	text = humanizeBotMention(text, &bot.Self)

	cid := getCID(msg)
	chatTitle := getChatTitle(msg, sender)
	order := getOrder(text, orders)
	config := getOrderConfig(conf, order)

	// get last line
	lastLine := toLine(text, sender, order)
	// get previous line (no order, as it's about stateless communication)
	var prevLine string
	if msg.ReplyToMessage != nil {
		temp := NewTgInfo(bot, msg.ReplyToMessage)
		prevLine = toLine(temp.Text, temp.Sender, "")
	} else {
		prevLine = ""
	}

	chatInfo := &ChatInfo{
		TgInfo: *t,
		Lines: []string{lastLine, prevLine},
		CID: cid,
		ChatTitle: chatTitle,
		Config: config,
		Order: order,
		MemLim: lim,
	}

	return chatInfo
}

func toLine(text string, name string, order string) string {
	var result string

	// empty text -> empty line
	if text == "" {
		return ""
	}

	// strip order if any, return text OR get name, return "Name: text" line
	if order != "" {
		text = strings.Replace(text, order, "", -1)
		result = text
	} else {
		result = name + ": " + text
	}

	return result
}

func getText(msg *tg.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Text != "" {
		return msg.Text
	}
	if msg.Caption != "" {
		return msg.Caption
	}
	return ""
}

// substitutes bot's user name mention to bot's first name addressing in text
func humanizeBotMention(text string, self *tg.User) string {
	botName, botFirstName := self.UserName, self.FirstName
	text = strings.Replace(text, "@"+botName, botFirstName+",", -1)
	return text
}

// gets name if any; or sets "anonymous"
func getName(msg *tg.Message, capitalize bool) string {
	var name string

	// bot -> first name, user -> user name
	if msg.From.IsBot {
		name = msg.From.FirstName
	} else {
		name = msg.From.UserName
	}

	// hidden user -> anonymous
	if name == "" {
		name = "anonymous"
	}

	// capitalize if asked
	if capitalize {
		caser := cases.Title(language.English)
		name = caser.String(name)
	}

	return name
}

// gets Chat ID for public and private chats
func getCID(msg *tg.Message) int64 {
	var cid int64
	if msg.Chat != nil {
		cid = msg.Chat.ID
	} else {
		cid = msg.From.ID
	}
	return cid
}

// gets chat title if any; or sets "User's chat"
func getChatTitle(msg *tg.Message, name string) string {
	chatName := msg.Chat.Title
	if chatName == "" {
		chatName = fmt.Sprintf("%s's chat", name)
	}
	return chatName
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

// adds order to bot config as postfix to specify order config
func getOrderConfig(botConfig string, order string) string {
	configPostfix := strings.Replace(order, "/", "_", 1)
	msgConfig := fmt.Sprintf(botConfig, configPostfix)
	return msgConfig
}

// checks if bot is asked; gets order if any
func Inspect(c *ChatInfo, admins []string) bool {
	bot, msg, text, order := c.Bot, c.Msg, c.Text, c.Order

	// get chat variable
	chat := msg.Chat

	// get bot's user name and ID
	self := bot.Self
	botName, botID := self.UserName, self.ID

	// get user's user name for correct admin check
	userName := getName(msg, false)

	// get replied message if any for reply check
	replied := msg.ReplyToMessage
	var repliedID int64 = 0
	if replied != nil {
		repliedID = replied.From.ID
	}

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

	return isAsked
}

// tries to reply the message with text; sends separate message on failure
func Reply(c *ChatInfo, text string) *tg.Message {
	bot, msg, cid := c.Bot, c.Msg, c.CID

	msgConf := tg.NewMessage(cid, text)
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
func Typing(ctx context.Context, c *ChatInfo) {
	bot, cid := c.Bot, c.CID
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
