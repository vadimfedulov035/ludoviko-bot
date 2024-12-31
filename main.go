package main

import (
	"fmt"
	"context"
	"os"
	"encoding/json"
	"log"

	tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"ludoviko-bot/api"
	"ludoviko-bot/history"
	"ludoviko-bot/memory"
	"ludoviko-bot/messaging"
)


const InitConf  = "./init.json"


type InitJSON struct {
	KeyAPI  string   `json:"keyAPI"`
	Admins  []string `json:"admins"`
	Confs   []string `json:"confs"`
	Orders  []string `json:"orders"`
	History string   `json:"history"`
	Limit   int      `json:"lim"`
}


func loadInitJSON(conf string) InitJSON {
	var initJSON InitJSON
	data, err := os.ReadFile(conf)
	if err != nil {
		panic(err)
	} else {
		json.Unmarshal(data, &initJSON)
	}
	return initJSON
}


func getConfig(order string, confs []string, orders []string) string {
	config := ""

	for _, conf_temp := range(confs) {
		for _, order_temp := range(orders) {
			if order_temp == order {
				config = conf_temp
				break
			}
		}
	}

	return config
}


func replyAI(bot *tg.BotAPI, msg *tg.Message,
					chatHistory map[string]string, limit int,
					conf string, order string) map[string]string {
    user := msg.From.UserName
	var userPrompt string
	if order != "" {
		userPrompt = msg.Text
	} else {
		userPrompt = memory.Remember(msg, chatHistory, limit)
	}
	chatDesc := msg.Chat.Description

	if chatHistory == nil {
		chatHistory = make(map[string]string)
	}

	requestBody := api.NewRequestBody(user, userPrompt, conf, order)
	requestBody.Settings.SystemPrompt = fmt.Sprintf(requestBody.Settings.SystemPrompt, chatDesc)

	text, err := api.SendToAPI(requestBody)
	if err != nil {
		log.Printf("%v", err)
	}
    response := messaging.Reply(bot, msg, text)

    memory.Memorize(response, chatHistory)

	return chatHistory
}


func replyTyping(bot *tg.BotAPI, msg *tg.Message,
				chatHistory map[string]string, limit int, id int64,
				conf string, order string) map[string]string {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go messaging.Typing(bot, id, ctx)

	chatHistory = replyAI(bot, msg, chatHistory, limit, conf, order)

	return chatHistory
}


func main() {
	initJSON := loadInitJSON(InitConf)
	KeyAPI,  Admins := initJSON.KeyAPI,  initJSON.Admins
	Confs,   Orders := initJSON.Confs,   initJSON.Orders
	History, Limit  := initJSON.History, initJSON.Limit

	bot, err := tg.NewBotAPI(KeyAPI)
	if err != nil {
		panic(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	chatsHistory := history.LoadHistory(History)

	u := tg.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)
	for update := range updates {
		msg := update.Message
		if msg == nil || msg.Text == ""  {
			continue
		}

		isAsked, isPrivate, isOrdered := messaging.IsToReply(msg, &bot.Self,
															Admins, Orders)
		if !isAsked {
			continue
		}

		var id int64
		if isPrivate {
			id = msg.From.ID
		} else {
			id = msg.Chat.ID
		}

		order := ""
		if isOrdered {
			order, _ = messaging.GetOrder(msg.Text, Orders)
		}

		conf := getConfig(order, Confs, Orders)
		chatsHistory[id] = replyTyping(bot, msg,
										chatsHistory[id], Limit, id,
										conf, order)

		history.WriteHistory(History, chatsHistory)
	}
}
