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

	var dialog []string
	// add message pair to history if replied message exists
	memory.Add(c, history, mu)
	// get dialog by going backwards in history via reply chain (2 lines)
	// else dialog is a new chain message (1 line)
	dialog = memory.Get(c, history, mu)

	// send dialog to API and reply with received text
	text := api.Send(dialog, c.Config, c.ChatTitle)
	resp := messaging.Reply(c, text)

	// add response pair to history
	c.Msg = resp
	memory.Add(c, history, mu)
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

	// get general constants
	Admins := initJSON.Admins
	Orders := initJSON.Orders[botName]
	MemLim := initJSON.MemoryLimit
	HistoryPath := initJSON.HistoryPath
	ConfigPath := initJSON.ConfigPath

	// get general history and config variables
	BotHistory := memory.GetBotHistory(history, botName)
	BotConfig := filepath.Join(ConfigPath, botName+"%s.json")

	// start update channel
	u := tg.NewUpdate(0)
	u.Timeout = 30
	updates := bot.GetUpdatesChan(u)
	for update := range updates {
		msg := update.Message

		// check if message is valid (via tgInfo)
		tgInfo := messaging.NewTgInfo(bot, msg)
		if tgInfo == nil || tgInfo.Text == "" {
			continue
		}

		// check if message is to current bot (via ChatInfo)
		chatInfo := messaging.NewChatInfo(tgInfo, BotConfig, Orders, MemLim)
		isAsked := messaging.Inspect(chatInfo, Admins)
		if !isAsked {
			continue
		}
		log.Printf("%s got message", botName)

		// get chat history (via ChatInfo)
		chatHistory := memory.GetChatHistory(BotHistory, chatInfo.CID)

		// reply with all info
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
