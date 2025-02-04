package memory

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"ludoviko-bot/messaging"
)

type MessageEntry struct {
	Message   string    `json:"msg"`
	Timestamp time.Time `json:"ts"`
}
type ChatHistory map[string]MessageEntry
type BotHistory map[int64]ChatHistory
type History map[string]BotHistory

type ChatInfo struct {
	ChatsHistory ChatHistory
	CID          int64
	Order        string
	Config       string
	MemoryLimit  int
}

func NewChatInfo(h ChatHistory, i int64, o string, c string, l int) *ChatInfo {
	return &ChatInfo{
		ChatsHistory: h,
		CID:          i,
		Order:        o,
		Config:       c,
		MemoryLimit:  l,
	}
}

// gets chat history; creates if none
func GetChatHistory(botHistory BotHistory, id int64) ChatHistory {
	if _, ok := botHistory[id]; !ok {
		botHistory[id] = make(ChatHistory)
	}
	chatHistory := botHistory[id]
	return chatHistory
}

// gets bot history in any case; creates if none
func GetBotHistory(history History, botName string) BotHistory {
	if _, ok := history[botName]; !ok {
		history[botName] = make(BotHistory)
	}
	botHistory := history[botName]
	return botHistory
}

// converts message to string with conditions
func toString(bot *tg.BotAPI, msg *tg.Message, order string) string {
	var result string

	// get text if any; no text -> empty string
	text, ok := messaging.GetMsgText(msg)
	if !ok {
		return ""
	}

	// replace bot username to the first name
	botName, botFirstName := bot.Self.UserName, bot.Self.FirstName
	text = strings.Replace(text, "@"+botName, botFirstName+",", -1)

	// strip order if any; avoid dialog structure
	if order != "" {
		text = strings.Replace(text, order, "", -1)
		result = text
		// contruct dialog
	} else {
		userName := messaging.GetUserName(msg)
		result = userName + ": " + text
	}

	return result
}

// adds message content to chat's info history
func Add(tgInfo *messaging.TgInfo, chatInfo *ChatInfo) (string, string) {
	bot, msg := tgInfo.Bot, tgInfo.Msg
	chatHistory, order := chatInfo.ChatsHistory, chatInfo.Order

	// get related lines
	prevLine := toString(bot, msg.ReplyToMessage, order)
	lastLine := toString(bot, msg, order)

	// add related lines to history if got both
	if prevLine != "" { // lastLine non-empty (message checked)
		chatHistory[lastLine] = MessageEntry{
			Message:   prevLine,
			Timestamp: time.Now(),
		}
	}

	return prevLine, lastLine
}

// gets dialog from chat's info history
func Get(prevLine string, lastLine string, chatInfo *ChatInfo) []string {
	chatHistory := chatInfo.ChatsHistory
	memLim := chatInfo.MemoryLimit

	// append last line (and return if needed)
	lines := []string{lastLine}
	if prevLine == "" { // lastLine non-empty (message checked)
		return lines
	}
	// append previous line
	lines = append(lines, prevLine)

	// append previous lines
	lastLine = prevLine
	for i := 0; i < memLim-2; i++ {
		if messageEntry, ok := chatHistory[lastLine]; ok {
			log.Printf("%d messages remembered", i+1)

			prevLine = messageEntry.Message
			lines = append(lines, prevLine)
			lastLine = prevLine
		} else {
			break
		}
	}

	// reverse the lines to get a dialog
	reverse := func(lines []string) []string {
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
		return lines
	}
	dialog := reverse(lines)

	return dialog
}

// cleans all day old lines in every chat history
func CleanHistory(history History) {
	currentTime := time.Now()

	for _, botHistory := range history {
		for _, chatHistory := range botHistory {
			var linesToDelete []string

			for line, messageEntry := range chatHistory {
				if currentTime.Sub(messageEntry.Timestamp) > 24*time.Hour {
					linesToDelete = append(linesToDelete, line)
				}
			}

			for _, line := range linesToDelete {
				delete(chatHistory, line)
			}
		}
	}
}

// loads history (for share use)
func LoadHistory(source string) History {
	var history History

	// open file
	file, err := os.OpenFile(source, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(fmt.Errorf("[OS error] History file opening: %v", err))
	}
	defer file.Close()

	// read file
	data, err := os.ReadFile(source)
	if err != nil {
		panic(fmt.Errorf("[OS error] History reading: %v", err))
	}

	// decode JSON to history
	err = json.Unmarshal(data, &history)
	if err != nil {
		log.Println("[OS] History will be created")
		history = History{}
	} else {
		log.Println("[OS] History loaded")
	}

	return history
}

// saves history with mutex locking
func SaveHistory(dest string, history History, mu *sync.Mutex) {
	mu.Lock()
	defer mu.Unlock()

	// open file
	file, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(fmt.Errorf("[OS error] History file opening: %v", err))
	}
	defer file.Close()

	// encode history into JSON
	data, err := json.Marshal(history)
	if err != nil {
		panic(fmt.Errorf("[OS error] History marshalling: %v", err))
	}

	// write JSON data to file
	_, err = file.Write(data)
	if err != nil {
		panic(fmt.Errorf("[OS error] History writing: %v", err))
	}

	log.Println("[OS] History written")
}
