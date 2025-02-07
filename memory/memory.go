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

	"tg-handler/messaging"
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
	Config       string
	Order        string
	MemoryLimit  int
}

func NewChatInfo(h ChatHistory, conf string, ord string, lim int) *ChatInfo {
	return &ChatInfo{
		ChatsHistory: h,
		Config:       conf,
		Order:        ord,
		MemoryLimit:  lim,
	}
}

func GetChatHistory(botHistory BotHistory, id int64) ChatHistory {
	if _, ok := botHistory[id]; !ok {
		botHistory[id] = make(ChatHistory)
	}
	chatHistory := botHistory[id]
	return chatHistory
}

func GetBotHistory(history History, botName string) BotHistory {
	if _, ok := history[botName]; !ok {
		history[botName] = make(BotHistory)
	}
	botHistory := history[botName]
	return botHistory
}

func ToLine(bot *tg.BotAPI, msg *tg.Message, order string) string {
	var result string

	// get text if any; none -> empty line
	text, ok := messaging.GetMsgText(msg)
	if !ok {
		return ""
	}

	// replace bot user name mention to first name addressing in text
	text = messaging.HumanizeBotMention(text, &bot.Self)

	// strip order if any -> text; none -> full line with capitalized name;
	if order != "" {
		text = strings.Replace(text, order, "", -1)
		result = text
	} else {
		userName := messaging.GetUserName(msg, true)
		result = userName + ": " + text
	}


	return result
}

// adds message content to chat's info history
func Add(tgInfo *messaging.TgInfo, chatInfo *ChatInfo, mu *sync.RWMutex) []string {
	mu.Lock()
	defer mu.Unlock()

	// get bot, message and order for line formatting
	bot, msg := tgInfo.Bot, tgInfo.Msg
	order := chatInfo.Order

	// get chat history for adding to it
	chatHistory := chatInfo.ChatsHistory

	// format two inversed lines
	lastLine := ToLine(bot, msg, order)
	prevLine := ToLine(bot, msg.ReplyToMessage, order)

	// lastLine non-empty (as line of valid message)
	lines := []string{lastLine}
	// prevLine empty check -> add if inversed pair to history; skip
	if prevLine != "" {
		chatHistory[lastLine] = MessageEntry{
			Message:   prevLine,
			Timestamp: time.Now(),
		}
		lines = append(lines, prevLine)
	}

	return lines
}

// gets dialog from chat's info history
func Get(lines []string, chatInfo *ChatInfo, mu *sync.RWMutex) []string {
	mu.RLock()
	defer mu.RUnlock()

	// get two inversed lines (non-empty as checked)
	lastLine := lines[0]
	prevLine := lines[1]

	// get chat history and memory limit for backward dialog assembling
	chatHistory := chatInfo.ChatsHistory
	memLim := chatInfo.MemoryLimit

	// accumulate inversed lines going backwards in history via reply chain
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

	// reverse inversed lines to get a dialog
	reverse := func(lines []string) []string {
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
		return lines
	}
	dialog := reverse(lines)

	return dialog
}

// loads history (for shared use)
func LoadHistory(source string) History {
	var history History

	// open file
	file, err := os.OpenFile(source, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(fmt.Errorf("[OS error] History file opening: %v", err))
	}
	defer file.Close()

	// read JSON data from file
	data, err := os.ReadFile(source)
	if err != nil {
		panic(fmt.Errorf("[OS error] History reading: %v", err))
	}

	// decode JSON data to history
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
func SaveHistory(dest string, history History, mu *sync.RWMutex) {
	mu.Lock()
	defer mu.Unlock()

	// open file
	file, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(fmt.Errorf("[OS error] History file opening: %v", err))
	}
	defer file.Close()

	// encode history to JSON data
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

// cleans all lines older than day in every chat history
func CleanHistory(history History, mu *sync.RWMutex) {
	mu.Lock()
	defer mu.Unlock()

	currentTime := time.Now()

	for _, botHistory := range history {
		for _, chatHistory := range botHistory {
			var linesToDelete []string

			for line, messageEntry := range chatHistory {
				if currentTime.Sub(messageEntry.Timestamp) > 24 * time.Hour {
					linesToDelete = append(linesToDelete, line)
				}
			}

			for _, line := range linesToDelete {
				delete(chatHistory, line)
			}
		}
	}
}
