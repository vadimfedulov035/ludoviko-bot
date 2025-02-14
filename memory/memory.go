package memory

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
	"strings"
	
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)


// interface providing lines for memory
type Liner interface {
	GetText() string
	GetSender() string
	GetOrder() string
}

// message storing format in history
type MessageEntry struct {
	Line      string    `json:"msg"`
	Timestamp time.Time `json:"ts"`
}
// chat history structure layers
type ChatHistory map[string]MessageEntry
type BotHistory map[int64]ChatHistory
type History map[string]BotHistory

// chat history getter
func GetChatHistory(botHistory BotHistory, id int64) ChatHistory {
	if _, ok := botHistory[id]; !ok {
		botHistory[id] = make(ChatHistory)
	}
	chatHistory := botHistory[id]
	return chatHistory
}

// bot history getter
func GetBotHistory(history History, botName string) BotHistory {
	if _, ok := history[botName]; !ok {
		history[botName] = make(BotHistory)
	}
	botHistory := history[botName]
	return botHistory
}

// returns text as line
func toLine(text string, sender string, order string) string {
	var result string

	// empty text -> empty line
	if text == "" {
		return ""
	}

	// capitalize sender
	capitalize := func(name string) string {
		caser := cases.Title(language.English)
		nameCap := caser.String(name)
		return nameCap
	}

	// strip order from string
	strip := func(text string, order string) string {
		return strings.Replace(text, order, "", -1)
	}

	// order implies anonymous line "text" (stripped order)
	// no order implies ordinary line "Name: text"
	if order != "" {
		result = strip(text, order)
	} else {
		result = capitalize(sender) + ": " + text
	}

	return result
}

// get lines via interface array or reused string
func getLines(ls [2]Liner, prevLine string) []string {
	// add last line to lines
	lastL := ls[0]
	text, sender, order := lastL.GetText(), lastL.GetSender(), lastL.GetOrder()
	lastLine := toLine(text, sender, order)
	lines := []string{lastLine}

	// add previous line to lines (old)
	if prevLine != "" {
		lines = append(lines, prevLine)
		return lines
	}

	// add previous line to lines (new)
	prevL := ls[1]
	prevText, prevSender, prevOrder := prevL.GetText(), prevL.GetSender(), ""
	// empty text or sender -> return
	if prevText == "" || prevSender == "" {
		return lines
	}
	// set to passed zero-string
	prevLine = toLine(prevText, prevSender, prevOrder)
	lines = append(lines, prevLine)

	return lines
}

// adds lines pair generated on the fly to chat history, returns lines
func Add(ls [2]Liner, line string, history ChatHistory, mu *sync.RWMutex) []string {
	mu.Lock()
	defer mu.Unlock()

	// make new or renew with passed line
	lines := getLines(ls, line)

	// got no pair to add, return
	if len(lines) < 2 {
		return lines
	}

	// get two inversed lines
	lastLine := lines[0]
	prevLine := lines[1]

	// add inversed lines to chat history
	history[lastLine] = MessageEntry{
		Line: prevLine,
		Timestamp: time.Now(),
	}

	return lines
}

// gets dialog from chat history
func Get(lines []string, history ChatHistory, memLim int, mu *sync.RWMutex) []string {
	mu.RLock()
	defer mu.RUnlock()

	// got no pair to decipher
	if len(lines) < 2 {
		return lines
	}

	// get two inversed lines
	lastLine := lines[0]
	prevLine := lines[1]

	// accumulate inversed lines going backwards in history via reply chain
	lastLine = prevLine
	for i := 0; i < memLim-2; i++ {
		if messageEntry, ok := history[lastLine]; ok {
			log.Printf("%d messages remembered", i+1)

			prevLine = messageEntry.Line
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
