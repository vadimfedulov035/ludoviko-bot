package messaging

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// message info (for validation check)
type MsgInfo struct {
	Bot *tg.BotAPI
	Msg *tg.Message
	Sender string
	Text string
}

// request info (for request check)
type ReqInfo struct {
	MsgInfo
	Order string
	Config string
}

// chat info (for further operatins)
type ChatInfo struct {
	ReqInfo
	CID int64
	ChatTitle string
	MemLim int
}

// message info constructor
func NewMsgInfo(bot *tg.BotAPI, msg *tg.Message) *MsgInfo {
	// nil message -> nil message info
	if msg == nil {
		return nil
	}

	return &MsgInfo{
		Bot: bot,
		Msg: msg,
		Sender: getName(msg),
		Text: getText(bot, msg),
	}
}

// request info constructor 
func NewReqInfo(m *MsgInfo, conf string, orders []string) *ReqInfo {
	text := m.Text

	// get order and config to set
	order := getOrder(text, orders)
	config := getOrderConfig(conf, order)

	return &ReqInfo{
		MsgInfo: *m,
		Order: order,
		Config: config,
	}
}

// chat info constructor
func NewChatInfo(r *ReqInfo, memLim int) *ChatInfo {
	// set recent message info
	msg := r.Msg
	sender := r.Sender

	return &ChatInfo{
		ReqInfo: *r,
		CID: getCID(msg),
		ChatTitle: getChatTitle(msg, sender),
		MemLim: memLim,
	}
}

// base getters for liner interface
// ()
// (chat info struct inherits redefined request info method)
func (m *MsgInfo) GetText() string { return m.Text }
func (m *MsgInfo) GetSender() string { return m.Sender }
func (r *MsgInfo) GetOrder() string { return "" }
func (r *ReqInfo) GetOrder() string { return r.Order }

// get text from message (always humanized)
func getText(bot *tg.BotAPI, msg *tg.Message) string {
	if msg == nil {
		return ""
	}

	// get any text
	var text string
	if msg.Text != "" {
		text = msg.Text
	} else if msg.Caption != "" {
		text = msg.Caption
	}

	// substitute @name_bot to BotName for humanized style
	// both form of addressing are detectable but only humanized will be passed
	humanize := func(text string, self *tg.User) string {
		botName, botFirstName := self.UserName, self.FirstName
		text = strings.Replace(text, "@"+botName, botFirstName+",", -1)
		return text
	}
	text = humanize(text, &bot.Self)

	return text
}

// gets name if any; or sets "anonymous"
func getName(msg *tg.Message) string {
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

// checks if bot is asked
func IsAsked(c *ReqInfo, admins []string) bool {
	bot, msg := c.Bot, c.Msg
	text, sender := c.Text, c.Sender
	order := c.Order

	// get chat variable
	chat := msg.Chat

	// get bot's first name and ID
	self := bot.Self
	botFirstName, botID := self.FirstName, self.ID

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
	isMentioned := strings.Contains(text, botFirstName)
	isOrdered := order != ""
	isAdmin := slices.Contains(admins, sender)

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

	// try to reply twice: as reply, as separate message
	response, err := bot.Send(msgConf)
	if err != nil {
		msgConf.ReplyToMessageID = 0
		response, err = bot.Send(msgConf)
	}
	// log final error
	if err != nil {
		log.Printf("[Telegram] Replying: %v", err)
	}

	return &response
}

// types every 3 seconds until context done
func Typing(ctx context.Context, c *ChatInfo) {
	bot, cid := c.Bot, c.CID

	// set 3 second ticker
	t := time.NewTicker(3 * time.Second)
	// stop on loop break
	defer t.Stop()

	for {
		select {
		case <-t.C:  // send typing signal on every tick
			actConf := tg.NewChatAction(cid, "typing")
			_, err := bot.Request(actConf)
			if err != nil {
				log.Printf("[Telegram] Action: %v", err)
			}
		case <-ctx.Done():  // break with return on context done
			return
		}
	}
}
