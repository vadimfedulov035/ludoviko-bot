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

func reply(tgInfo *messaging.TgInfo, chatInfo *memory.ChatInfo, mu *sync.RWMutex) {
	// type until reply
	ctx, cancel := context.WithCancel(context.Background())
	go messaging.Typing(ctx, tgInfo)
	defer cancel()

	var dialog []string
	// add message pair to history if replied message exists, return as lines
	lines := memory.Add(tgInfo, chatInfo, mu)
	// get dialog going backwards in history via reply chain (2 lines)
	if len(lines) > 1 {
		dialog = memory.Get(lines, chatInfo, mu)
	} else {
		dialog = lines // else dialog is a new chain message (1 line)
	}

	// send dialog to API and reply with received text
	text := api.Send(dialog, chatInfo.Config, tgInfo.Msg.Chat.Title)
	resp := messaging.Reply(tgInfo, text)

	// add response pair to history
	tgInfo.Msg = resp
	memory.Add(tgInfo, chatInfo, mu)
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
	Config := filepath.Join(ConfigPath, botName+"%s.json")

	// start update channel
	u := tg.NewUpdate(0)
	u.Timeout = 30
	updates := bot.GetUpdatesChan(u)
	for update := range updates {
		msg := update.Message

		// check if message is valid
		_, ok := messaging.GetMsgText(msg)
		if !ok {
			continue
		}

		// get Chat ID -> tgInfo
		cid := messaging.GetCID(msg)
		tgInfo := messaging.NewTgInfo(bot, msg, cid)

		// check if message is to current bot, log success
		isAsked, order := messaging.Inspect(tgInfo, Admins, Orders)
		if !isAsked {
			continue
		}
		log.Printf("%s got message", botName)

		// get chat history, custom bot config -> chatInfo
		chatHistory := memory.GetChatHistory(BotHistory, cid)
		config := messaging.GetCustomConfig(Config, order)
		chatInfo := memory.NewChatInfo(chatHistory, config, order, MemLim)

		// reply with all info
		reply(tgInfo, chatInfo, mu)

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
