package main


import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"

	tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"ludoviko-bot/api"
	"ludoviko-bot/memory"
	"ludoviko-bot/messaging"
)


const InitConf = "./init.json"


type InitJSON struct {
	KeysAPI     []string            `json:"keysAPI"`
	Admins      []string            `json:"admins"`
	Orders      map[string][]string `json:"orders"`
	ConfigPath  string              `json:"config_path"`
	HistoryPath string              `json:"history_path"`
	MemoryLimit int                 `json:"memory_limit"`
}


func loadInitJSON(conf string) *InitJSON {
	var initJSON InitJSON

	data, err := os.ReadFile(conf)
	if err != nil {
			panic(err)
	}

	err = json.Unmarshal(data, &initJSON)
	if err != nil {
		panic(err)
	}

	return &initJSON
}


func reply(tgInfo *messaging.TgInfo, chatInfo *memory.ChatInfo) {
        bot, msg := tgInfo.Bot, tgInfo.Msg
        cid, config := chatInfo.CID, chatInfo.Config

        chatName := msg.Chat.Title

        // type until reply
        ctx, cancel := context.WithCancel(context.Background())
        go messaging.Typing(ctx, bot, cid)
        defer cancel()

        // add user response to history
        prevLine, lastLine := memory.Add(tgInfo, chatInfo)

        // get dialog
        dialog := memory.Get(prevLine, lastLine, chatInfo)

        // send dialog to API and reply with received text
        text := api.Send(dialog, config, chatName)
        resp := messaging.Reply(bot, msg, text)

        // add bot response to history
        tgInfo = messaging.NewTgInfo(bot, resp)
        memory.Add(tgInfo, chatInfo)
}


func start(i int, initJSON *InitJSON, history memory.History, mu *sync.Mutex) {
        keysAPI := initJSON.KeysAPI

        // start bot
        bot, err := tg.NewBotAPI(keysAPI[i])
        if err != nil {
                panic(err)
        }
        botName := bot.Self.UserName
        log.Printf("Authorized on account %s", botName)

        // get base constants
        Admins := initJSON.Admins
        Orders := initJSON.Orders[botName]
        MemLim := initJSON.MemoryLimit
        HistoryPath := initJSON.HistoryPath
        ConfigPath := initJSON.ConfigPath

        // get general history and config variables
        BotHistory := memory.GetBotHistory(history, botName)
        Config := filepath.Join(ConfigPath, botName + "%s.json")

        // preemptively clean history
        memory.CleanHistory(history)
        memory.SaveHistory(HistoryPath, history, mu)

        u := tg.NewUpdate(0)
        u.Timeout = 30
        updates := bot.GetUpdatesChan(u)
        for update := range updates {
                msg := update.Message

                // check if message is not null, has text/caption
                _, ok := messaging.GetMsgText(msg)
                if !ok {
                        continue
                }

                // check if message is to current bot (+ tgInfo)
                tgInfo := messaging.NewTgInfo(bot, msg)
                isAsked, order := messaging.Inspect(tgInfo, Admins, Orders)
                if !isAsked {
                        continue
                }
                log.Printf("%s got message", botName)

                // get chat history, custom bot config (+ chatInfo)
                cid := messaging.GetCID(msg)
                chatHistory := memory.GetChatHistory(BotHistory, cid)
                config := messaging.GetCustomConfig(Config, order)
                chatInfo := memory.NewChatInfo(chatHistory, cid, order, config, MemLim)

                // reply with all info
                reply(tgInfo, chatInfo)

                // clean and save history
                memory.CleanHistory(history)
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
        var mu sync.Mutex

        // start all bots with shared history and mutex
        for i := range KeysAPI {
                go start(i, initJSON, history, &mu)
        }

        select{}
}
