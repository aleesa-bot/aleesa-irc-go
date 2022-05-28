package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/go-redis/redis/v8"
	"github.com/hjson/hjson-go"
	log "github.com/sirupsen/logrus"
	irc "github.com/thoj/go-ircevent"
)

// Конфиг
type myConfig struct {
	Redis struct {
		Server    string `json:"server,omitempty"`
		Port      int    `json:"port,omitempty"`
		Channel   string `json:"channel,omitempty"`
		MyChannel string `json:"my_channel,omitempty"`
	} `json:"redis"`
	Irc struct {
		Server    string   `json:"server,omitempty"`
		Port      int      `json:"port,omitempty"`
		Ssl       bool     `json:"ssl,omitempty"`
		SslVerify bool     `json:"ssl_verify,omitempty"`
		Nick      string   `json:"nick,omitempty"`
		User      string   `json:"user,omitempty"`
		Password  string   `json:"password,omitempty"`
		Channels  []string `json:"channels"`
	}
	Loglevel    string `json:"loglevel,omitempty"`
	Log         string `json:"log,omitempty"`
	Csign       string `json:"csign,omitempty"`
	ForwardsMax int64  `json:"forwards_max,omitempty"`
}

// Входящее сообщение из pubsub-канала redis-ки
type rMsg struct {
	From    string `json:"from,omitempty"`
	Chatid  string `json:"chatid,omitempty"`
	Userid  string `json:"userid,omitempty"`
	Message string `json:"message,omitempty"`
	Plugin  string `json:"plugin,omitempty"`
	Mode    string `json:"mode,omitempty"`
	Misc    struct {
		Answer      int64  `json:"answer,omitempty"`
		Botnick     string `json:"bot_nick,omitempty"`
		Csign       string `json:"csign,omitempty"`
		Fwdcnt      int64  `json:"fwd_cnt,omitempty"`
		GoodMorning int64  `json:"good_morning,omitempty"`
		Msgformat   int64  `json:"msg_format,omitempty"`
		Username    string `json:"username,omitempty"`
	} `json:"Misc"`
}

// Исходящее сообщение в pubsub-канал redis-ки
type sMsg struct {
	From    string `json:"from"`
	Chatid  string `json:"chatid"`
	Userid  string `json:"userid"`
	Message string `json:"message"`
	Plugin  string `json:"plugin"`
	Mode    string `json:"mode"`
	Misc    struct {
		Answer      int64  `json:"answer"`
		Botnick     string `json:"bot_nick"`
		Csign       string `json:"csign"`
		Fwdcnt      int64  `json:"fwd_cnt"`
		GoodMorning int64  `json:"good_morning"`
		Msgformat   int64  `json:"msg_format"`
		Username    string `json:"username"`
	} `json:"misc"`
}

// Config - это у нас глобальная штука :)
var config myConfig

// To break circular message forwarding we must set some sane default, it can be overridden via config
var forwardMax int64 = 5

// Объектики irc-клиента
var ircClient *irc.Connection

// Объектики клиента-редиски
var redisClient *redis.Client
var subscriber *redis.PubSub

// Main context
var ctx = context.Background()

// Ставится в true, если мы получили сигнал на выключение
var shutdown = false

// Канал, в который приходят уведомления для хэндлера сигналов от траппера сигналов
var sigChan = make(chan os.Signal, 1)

// Читает и валидирует конфиг, а также выставляет некоторые default-ы, если значений для параметров в конфиге нет
func readConfig() {
	configLoaded := false
	executablePath, err := os.Executable()

	if err != nil {
		log.Errorf("Unable to get current executable path: %s", err)
	}

	configJSONPath := fmt.Sprintf("%s/data/config.json", filepath.Dir(executablePath))

	locations := []string{
		"~/.aleesa-misc-go.json",
		"~/aleesa-misc-go.json",
		"/etc/aleesa-misc-go.json",
		configJSONPath,
	}

	for _, location := range locations {
		fileInfo, err := os.Stat(location)

		// Предполагаем, что файла либо нет, либо мы не можем его прочитать, второе надо бы логгировать, но пока забьём
		if err != nil {
			continue
		}

		// Конфиг-файл длинноват для конфига, попробуем следующего кандидата
		if fileInfo.Size() > 65535 {
			log.Warnf("Config file %s is too long for config, skipping", location)
			continue
		}

		buf, err := ioutil.ReadFile(location)

		// Не удалось прочитать, попробуем следующего кандидата
		if err != nil {
			log.Warnf("Skip reading config file %s: %s", location, err)
			continue
		}

		// Исходя из документации, hjson какбы умеет парсить "кривой" json, но парсит его в map-ку.
		// Интереснее на выходе получить структурку: то есть мы вначале конфиг преобразуем в map-ку, затем эту map-ку
		// сериализуем в json, а потом json преврщааем в стркутурку. Не очень эффективно, но он и не часто требуется.
		var sampleConfig myConfig
		var tmp map[string]interface{}
		err = hjson.Unmarshal(buf, &tmp)

		// Не удалось распарсить - попробуем следующего кандидата
		if err != nil {
			log.Warnf("Skip parsing config file %s: %s", location, err)
			continue
		}

		tmpjson, err := json.Marshal(tmp)

		// Не удалось преобразовать map-ку в json
		if err != nil {
			log.Warnf("Skip parsing config file %s: %s", location, err)
			continue
		}

		if err := json.Unmarshal(tmpjson, &sampleConfig); err != nil {
			log.Warnf("Skip parsing config file %s: %s", location, err)
			continue
		}

		// Валидируем значения из конфига
		if sampleConfig.Redis.Server == "" {
			sampleConfig.Redis.Server = "localhost"
		}

		if sampleConfig.Redis.Port == 0 {
			sampleConfig.Redis.Port = 6379
		}

		if sampleConfig.Redis.Channel == "" {
			log.Errorf("Channel field in config file %s must be set", location)
			os.Exit(1)
		}

		if sampleConfig.Redis.MyChannel == "" {
			log.Errorf("My_channel field in config file %s must be set", location)
			os.Exit(1)
		}

		// Значения для IRC-клиента
		if sampleConfig.Irc.Server == "" {
			sampleConfig.Irc.Server = "localhost"
			log.Errorf("Irc server is not defined in config, using localhost")
		}

		if sampleConfig.Irc.Port == 0 {
			sampleConfig.Irc.Port = 6667
			log.Infof("Irc port is not defined in config, using 6667")
		}

		if sampleConfig.Irc.Ssl != true {
			sampleConfig.Irc.SslVerify = false
		}

		if sampleConfig.Irc.Nick == "" {
			log.Errorf("Irc nick is not defined in config, quitting")
			os.Exit(1)
		}

		if sampleConfig.Irc.User == "" {
			sampleConfig.Irc.User = sampleConfig.Irc.Nick
		}

		// Если sampleConfig.Irc.Password не задан, то авторизации через Nickserv не будет

		// Нам бот нужен на каких-то IRC-каналах, а не "просто так"
		if len(sampleConfig.Irc.Channels) < 1 {
			log.Errorf("No irc channels defined in config, quitting")
			os.Exit(1)
		}

		if sampleConfig.Loglevel == "" {
			sampleConfig.Loglevel = "info"
		}

		// sampleConfig.Log = "" if not set

		if sampleConfig.Csign == "" {
			log.Errorf("Csign field in config file %s must be set", location)
		}

		if sampleConfig.ForwardsMax == 0 {
			sampleConfig.ForwardsMax = forwardMax
		}

		config = sampleConfig
		configLoaded = true
		log.Infof("Using %s as config file", location)
		break
	}

	if !configLoaded {
		log.Error("Config was not loaded! Refusing to start.")
	}
}

// Хэндлер сигналов закрывает все бд, все сетевые соединения и сваливает из приложения
func sigHandler() {
	var err error

	for {
		var s = <-sigChan
		switch s {
		case syscall.SIGINT:
			log.Infoln("Got SIGINT, quitting")
		case syscall.SIGTERM:
			log.Infoln("Got SIGTERM, quitting")
		case syscall.SIGQUIT:
			log.Infoln("Got SIGQUIT, quitting")

		// Заходим на новую итерацию, если у нас "неинтересный" сигнал
		default:
			continue
		}

		// Чтобы не срать в логи ошибками от редиски, проставим shutdown state приложения в true
		shutdown = true

		// Отпишемся от всех каналов и закроем коннект к редиске
		if err = subscriber.Unsubscribe(ctx); err != nil {
			log.Errorf("Unable to unsubscribe from redis channels cleanly: %s", err)
		}

		if err = subscriber.Close(); err != nil {
			log.Errorf("Unable to close redis connection cleanly: %s", err)
		}

		log.Info("Quitting")
		ircClient.Quit()
		os.Exit(0)
	}
}

// msgParser парсит json-чики прилетевшие из REDIS-ки
func msgParser(msg string) {
	return
}

// ircClientInit горутинка для работы с протоколом irc
func ircClientRun() {
	for {
		// Иницализируем irc-клиента
		ircClient = irc.IRC(config.Irc.Nick, config.Irc.User)

		if config.Irc.Ssl {
			ircClient.UseTLS = true

			if !config.Irc.SslVerify {
				ircClient.TLSConfig = &tls.Config{InsecureSkipVerify: true}
			}
		}

		// Навесим коллбэков на все возможные и невозможные error status code, которые мы можем получить и сдампим это
		// дело в лог. https://datatracker.ietf.org/doc/html/rfc1459 и https://datatracker.ietf.org/doc/html/rfc2812
		ircClient.AddCallback("401", func(e *irc.Event) {
			log.Errorf("401 ERR_NOSUCHNICK, %s", e.Raw)
		})
		ircClient.AddCallback("403", func(e *irc.Event) {
			log.Errorf("403 ERR_NOSUCHCHANNEL, %s", e.Raw)
		})
		ircClient.AddCallback("404", func(e *irc.Event) {
			log.Errorf("404 ERR_CANNOTSENDTOCHAN, %s", e.Raw)
		})
		ircClient.AddCallback("405", func(e *irc.Event) {
			log.Errorf("405 ERR_TOOMANYCHANNELS, %s", e.Raw)
		})
		ircClient.AddCallback("407", func(e *irc.Event) {
			log.Errorf("407 ERR_TOOMANYTARGETS, %s", e.Raw)
		})
		ircClient.AddCallback("411", func(e *irc.Event) {
			log.Errorf("411 ERR_NORECIPIENT, %s", e.Raw)
		})
		ircClient.AddCallback("412", func(e *irc.Event) {
			log.Errorf("412 ERR_NOTEXTTOSEND, %s", e.Raw)
		})
		ircClient.AddCallback("421", func(e *irc.Event) {
			log.Errorf("421 ERR_UNKNOWNCOMMAND, %s", e.Raw)
		})
		ircClient.AddCallback("431", func(e *irc.Event) {
			log.Errorf("431 ERR_NONICKNAMEGIVEN, %s", e.Raw)
		})
		ircClient.AddCallback("432", func(e *irc.Event) {
			log.Errorf("432 ERR_ERRONEUSNICKNAME, %s", e.Raw)
		})
		ircClient.AddCallback("433", func(e *irc.Event) {
			log.Errorf("433 ERR_NICKNAMEINUSE, %s", e.Raw)
		})
		ircClient.AddCallback("436", func(e *irc.Event) {
			log.Errorf("436 ERR_NICKCOLLISION, %s", e.Raw)
		})
		ircClient.AddCallback("437", func(e *irc.Event) {
			log.Errorf("437 ERR_UNAVAILRESOURCE, %s", e.Raw)
		})
		ircClient.AddCallback("441", func(e *irc.Event) {
			log.Errorf("441 ERR_USERNOTINCHANNEL, %s", e.Raw)
		})
		ircClient.AddCallback("442", func(e *irc.Event) {
			log.Errorf("442 ERR_NOTONCHANNEL, %s", e.Raw)
		})
		ircClient.AddCallback("451", func(e *irc.Event) {
			log.Errorf("451 ERR_NOTREGISTERED, %s", e.Raw)
		})
		ircClient.AddCallback("461", func(e *irc.Event) {
			log.Errorf("461 ERR_NEEDMOREPARAMS, %s", e.Raw)
		})
		ircClient.AddCallback("462", func(e *irc.Event) {
			log.Errorf("462 ERR_ALREADYREGISTRED, %s", e.Raw)
		})
		ircClient.AddCallback("464", func(e *irc.Event) {
			log.Errorf("464 ERR_PASSWDMISMATCH, %s", e.Raw)
		})
		ircClient.AddCallback("465", func(e *irc.Event) {
			log.Errorf("465 ERR_YOUREBANNEDCREEP, %s", e.Raw)
		})
		ircClient.AddCallback("471", func(e *irc.Event) {
			log.Errorf("471 ERR_CHANNELISFULL, %s", e.Raw)
		})
		ircClient.AddCallback("472", func(e *irc.Event) {
			log.Errorf("472 ERR_UNKNOWNMODE, %s", e.Raw)
		})
		ircClient.AddCallback("473", func(e *irc.Event) {
			log.Errorf("473 ERR_INVITEONLYCHAN, %s", e.Raw)
		})
		ircClient.AddCallback("474", func(e *irc.Event) {
			log.Errorf("474 ERR_BANNEDFROMCHAN, %s", e.Raw)
		})
		ircClient.AddCallback("477", func(e *irc.Event) {
			log.Errorf("477 ERR_NOCHANMODES, %s", e.Raw)
		})
		ircClient.AddCallback("478", func(e *irc.Event) {
			log.Errorf("478 ERR_BANLISTFULL, %s", e.Raw)
		})
		ircClient.AddCallback("482", func(e *irc.Event) {
			log.Errorf("482 ERR_CHANOPRIVSNEEDED, %s", e.Raw)
		})
		ircClient.AddCallback("484", func(e *irc.Event) {
			log.Errorf("484 ERR_RESTRICTED, %s", e.Raw)
		})
		ircClient.AddCallback("485", func(e *irc.Event) {
			log.Errorf("485 ERR_UNIQOPPRIVSNEEDED, %s", e.Raw)
		})
		ircClient.AddCallback("501", func(e *irc.Event) {
			log.Errorf("501 ERR_UMODEUNKNOWNFLAG, %s", e.Raw)
		})
		ircClient.AddCallback("502", func(e *irc.Event) {
			log.Errorf("502 ERR_USERSDONTMATCH, %s", e.Raw)
		})

		// По выходу из клиента
		ircClient.AddCallback("376", func(e *irc.Event) {
			log.Info("Quitting")
			ircClient.Quit()
		})

		// Сделаем уже что-то полезное!
		ircClient.AddCallback("001", func(e *irc.Event) {
			for _, channel := range config.Irc.Channels {
				ircClient.Join(channel)
			}
		})

		// Здесь у нас парсер сообщений из IRC
		ircClient.AddCallback("PRIVMSG", func(e *irc.Event) {
			return
		})
	}
}

// Производит некоторую инициализацию перед запуском main()
func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableQuote:           true,
		DisableLevelTruncation: false,
		DisableColors:          true,
		FullTimestamp:          true,
		TimestampFormat:        "2006-01-02 15:04:05",
	})

	readConfig()

	// no panic, no trace
	switch config.Loglevel {
	case "fatal":
		log.SetLevel(log.FatalLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}

func main() {
	// Main context
	var ctx = context.Background()

	// Откроем лог и скормим его логгеру
	if config.Log != "" {
		logfile, err := os.OpenFile(config.Log, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)

		if err != nil {
			log.Fatalf("Unable to open log file %s: %s", config.Log, err)
		}

		log.SetOutput(logfile)
	}

	// Иницализируем redis-клиента
	redisClient = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", config.Redis.Server, config.Redis.Port),
	})

	log.Debugf("Lazy connect() to redis at %s:%d", config.Redis.Server, config.Redis.Port)
	subscriber = redisClient.Subscribe(ctx, config.Redis.Channel)
	redisMsgChan := subscriber.Channel()

	go ircClientRun()

	// Самое время поставить траппер сигналов
	signal.Notify(sigChan,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go sigHandler()

	// Обработчик событий от редиски
	for msg := range redisMsgChan {
		msgParser(msg.Payload)
	}
}
