package main

import (
	"context"
	"os"

	"aleesa-irc-go/internal/anycollection"
	"aleesa-irc-go/internal/boolcollection"

	"github.com/cockroachdb/pebble"
	"github.com/go-redis/redis/v8"
	irc "github.com/thoj/go-ircevent"
)

// Config - это у нас глобальная штука :)
var config myConfig

// To break circular message forwarding we must set some sane default, it can be overridden via config
var forwardMax int64 = 5

// Объектики irc-клиента
var ircClient *irc.Connection
var nickIsUsed = false

// Объектики клиента-редиски
var redisClient *redis.Client
var subscriber *redis.PubSub

// Main context
var ctx = context.Background()

// Ставится в true, если мы получили сигнал на выключение
var shutdown = false

// Канал, в который приходят уведомления для хэндлера сигналов от траппера сигналов
var sigChan = make(chan os.Signal, 1)

// Канал, в который пишутся сообщения для отправки в IRC в обычном порядке
var imChan = make(chan iMsg, 10000)

// Канал, в который пишутся сообщения для отправки в IRC, если ограничений нет (у бота +o или +v на канале)
var imChanUnrestricted = make(chan iMsg, 100)

// "Ведёрко" с timestamp-ами последних отправленных сообщений
var msgBucket bucket

// Мапка с открытыми дескрипторами баз с настройками
var settingsDB = make(map[string]*pebble.DB)

// "Базюлька" с MODE-ами пользователей на каналах
var userMode = anycollection.NewCollection()

// "Базюлька" с доступными MODE-ами пользователя
var availableUserModes = boolcollection.NewCollection()

// "Базюлька" c с доступными MODE-ами каналов
var availableChanModes = boolcollection.NewCollection()

/* vim: set ft=go noet ai ts=4 sw=4 sts=4: */
