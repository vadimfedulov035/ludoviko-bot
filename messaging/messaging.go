package messaging


import (
    "context"
    "strings"
    "slices"
    "time"
    "log"

    tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)


func GetOrder(text string, orders []string) (string, bool) {
    order := ""

    for _, order := range(orders) {
        if order == "" {
            continue
        }

        if strings.Contains(text, order) {
            return order, true
        }
    }

    return order, false
}


func IsToReply(msg *tg.Message, self *tg.User, admins []string, orders []string) (bool, bool, bool) {
    chat := msg.Chat
    user := msg.From.UserName
    repliedMsg := msg.ReplyToMessage

    isPublic := chat.IsGroup() || chat.IsSuperGroup()
    isPrivate := chat.IsPrivate()

    isReplied := repliedMsg != nil && repliedMsg.From.ID == self.ID
    isMentioned := strings.Contains(msg.Text, self.UserName)
    _, isOrdered := GetOrder(msg.Text, orders)

    isAdmin := slices.Contains(admins, user)

    isAskedPublicly := isPublic && (isReplied || isMentioned || isOrdered)
    isAskedPrivately := isPrivate && isAdmin

    isAsked := isAskedPublicly || isAskedPrivately

    return isAsked, isAskedPrivately, isOrdered
}


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


func Typing(bot *tg.BotAPI, id int64, ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
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
