package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"

	tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"tg-handler/api"
	"tg-handler/memory"
	"tg-handler/messaging"
)

const InitConf = "./init/init.json"

type InitJSON struct {
	KeysAPI     []string            `json:"keysAPI"`
	Admins      []string            `json:"admins"`
	Orders      map[string][]string `json:"orders"`
	ConfigPath  string              `json:"config_path"`
	HistoryPath string              `json:"history_path"`
	MemoryLimit int                 `json:"memory_limit"`
}

func loadInitJSON(config string) *InitJSON {
	var initJSON InitJSON

	// read JSON data from file
	data, err := os.ReadFile(config)
	if err != nil {
		panic(err)
	}

	// decode JSON data to InitJSON
	err = json.Unmarshal(data, &initJSON)
	if err != nil {
		panic(err)
	}

	return &initJSON
}

func reply(c *messaging.ChatInfo, history memory.ChatHistory, mu *sync.RWMutex) {
	// type until reply
	ctx, cancel := context.WithCancel(context.Background())
	go messaging.Typing(ctx, c)
	defer cancel()

	// add old message pair to history, get it (both: interfaces)
	m := messaging.NewMsgInfo(c.Bot, c.Msg.ReplyToMessage)
	lines := memory.Add([2]memory.Liner{c, m}, "", history, mu)

	// get dialog, send to API and reply
	dialog := memory.Get(lines, history, c.MemLim, mu)
	text, err := api.Send(dialog, c.Config, c.ChatTitle)
	if err != nil {
		return
	}
	resp := messaging.Reply(c, text)

	// add new message pair to history (last: interface, previous: reused)
	m = messaging.NewMsgInfo(c.Bot, resp)
	memory.Add([2]memory.Liner{m, nil}, lines[0], history, mu)
}

func start(i int, initJSON *InitJSON, history memory.History, mu *sync.RWMutex) {
	// start bot from specific keyAPI
	keysAPI := initJSON.KeysAPI
	bot, err := tg.NewBotAPI(keysAPI[i])
	if err != nil {
		panic(err)
	}
	// log authorization
	botName := bot.Self.UserName
	log.Printf("Authorized on account %s", botName)

	// get initially specified constants
	Admins := initJSON.Admins
	Orders := initJSON.Orders[botName]
	MemLim := initJSON.MemoryLimit
	HistoryPath := initJSON.HistoryPath
	ConfigPath := initJSON.ConfigPath

	// get bot history and config
	BotHistory := memory.GetBotHistory(history, botName)
	BotConfig := filepath.Join(ConfigPath, botName+"%s.json")

	// start update channel
	u := tg.NewUpdate(0)
	u.Timeout = 30
	updates := bot.GetUpdatesChan(u)
	for update := range updates {
		msg := update.Message

		// check if message is valid (via msgInfo: Bot, Msg, Text, Sender)
		msgInfo := messaging.NewMsgInfo(bot, msg)
		if msgInfo == nil || msgInfo.Text == "" || msgInfo.Sender == "" {
			continue
		}

		// check if message is to current bot (via ReqInfo: Config, Order)
		reqInfo := messaging.NewReqInfo(msgInfo, BotConfig, Orders)
		isAsked := messaging.IsAsked(reqInfo, Admins)
		if !isAsked {
			continue
		}
		log.Printf("%s got message", botName)

		// get chat history (via ChatInfo: CID, ChatTitle)
		chatInfo := messaging.NewChatInfo(reqInfo, MemLim)
		chatHistory := memory.GetChatHistory(BotHistory, chatInfo.CID)

		// reply to message
		reply(chatInfo, chatHistory, mu)

		// clean and save history
		memory.CleanHistory(history, mu)
		memory.SaveHistory(HistoryPath, history, mu)
	}
}

func main() {
	// load initialization config
	initJSON := loadInitJSON(InitConf)

	// get KeysAPI and HistoryPath
	KeysAPI := initJSON.KeysAPI
	HistoryPath := initJSON.HistoryPath

	// load shared history and mutex
	history := memory.LoadHistory(HistoryPath)
	var mu sync.RWMutex

	// start all bots with shared history and mutex
	for i := range KeysAPI {
		go start(i, initJSON, history, &mu)
	}

	select {}
}
