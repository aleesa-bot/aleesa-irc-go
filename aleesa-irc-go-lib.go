package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/hjson/hjson-go"
	log "github.com/sirupsen/logrus"
	irc "github.com/thoj/go-ircevent"
)

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

		// Если sampleConfig.Irc.Password не задан, то авторизации через Nickserv или SASL не будет
		// Если sampleConfig.Irc.Sasl не задан, то авторизация происходит через NickServ

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
		os.Exit(1)
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
		} else {
			log.Debug("Unsubscribe from all redis channels")
		}

		if err = subscriber.Close(); err != nil {
			log.Errorf("Unable to close redis connection cleanly: %s", err)
		} else {
			log.Debug("Close redis connection")
		}

		log.Debug("Close irc connection")
		ircClient.Quit()
		os.Exit(0)
	}
}

// ircClientInit горутинка для работы с протоколом irc
func ircClientRun() {
	for {
		if shutdown {
			// Если мы завершаем работу программы, то нам ничего обрабатывать не надо
			break
		}

		// Иницализируем irc-клиента
		serverString := fmt.Sprintf("%s:%d", config.Irc.Server, config.Irc.Port)
		log.Debugf("Preparing to connect to %s", serverString)
		log.Debugf("Using nick %s and username %s", config.Irc.Nick, config.Irc.User)
		ircClient = irc.IRC(config.Irc.Nick, config.Irc.User)
		ircClient.RealName = config.Irc.User
		ircClient.Version = "Aleesa Bot v4.something"

		if config.Irc.Ssl {
			log.Debug("Force use ssl for connection")
			ircClient.UseTLS = true

			if !config.Irc.SslVerify {
				log.Debug("Skip server certificate validation")
				ircClient.TLSConfig = &tls.Config{InsecureSkipVerify: true}
			} else {
				log.Debug("Force server certificate validation")
			}
		} else {
			log.Debug("Skip ssl for connection")
		}

		if config.Irc.Sasl && config.Irc.Password != "" {
			ircClient.SASLLogin = config.Irc.User
			ircClient.SASLPassword = config.Irc.Password
			ircClient.UseSASL = true
		}

		// Навесим коллбэков на все возможные и невозможные error status code, которые мы можем получить и сдампим это
		// дело в лог. https://datatracker.ietf.org/doc/html/rfc1459 и https://datatracker.ietf.org/doc/html/rfc2812
		ircClient.AddCallback("401", func(e *irc.Event) {
			log.Warnf("401 ERR_NOSUCHNICK, %s", e.Raw)
		})
		ircClient.AddCallback("403", func(e *irc.Event) {
			log.Warnf("403 ERR_NOSUCHCHANNEL, %s", e.Raw)
		})
		ircClient.AddCallback("404", func(e *irc.Event) {
			log.Warnf("404 ERR_CANNOTSENDTOCHAN, %s", e.Raw)
		})
		ircClient.AddCallback("405", func(e *irc.Event) {
			// Тут мы наткнулись на ограничение сервера, сделать с этим мы ничего не можем
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
			nickIsUsed = true
			time.Sleep(60 * time.Second)
			ircClient.Nick(config.Irc.Nick)
		})
		ircClient.AddCallback("436", func(e *irc.Event) {
			// Что это за зверь такой?
			// Предположительно, тут имеется в виду ситуация, когда в конфедерации серверов ник был зареган на двух
			// серверах и теперь сервер не знает, что с этим делать
			log.Errorf("436 ERR_NICKCOLLISION, %s", e.Raw)
		})
		ircClient.AddCallback("437", func(e *irc.Event) {
			log.Errorf("437 ERR_UNAVAILRESOURCE, %s", e.Raw)
		})
		ircClient.AddCallback("441", func(e *irc.Event) {
			log.Warnf("441 ERR_USERNOTINCHANNEL, %s", e.Raw)
		})
		ircClient.AddCallback("442", func(e *irc.Event) {
			log.Warnf("442 ERR_NOTONCHANNEL, %s", e.Raw)
		})
		ircClient.AddCallback("451", func(e *irc.Event) {
			// Предполагается, что надо авторизоваться, перед тем как что-то делать на сервере
			log.Errorf("451 ERR_NOTREGISTERED, %s", e.Raw)
		})
		ircClient.AddCallback("461", func(e *irc.Event) {
			log.Errorf("461 ERR_NEEDMOREPARAMS, %s", e.Raw)
		})
		ircClient.AddCallback("462", func(e *irc.Event) {
			log.Warnf("462 ERR_ALREADYREGISTRED, %s", e.Raw)
		})
		ircClient.AddCallback("464", func(e *irc.Event) {
			log.Errorf("464 ERR_PASSWDMISMATCH, %s", e.Raw)
		})
		ircClient.AddCallback("465", func(e *irc.Event) {
			// Этот бан на сервере целиком, если верить rfc, здесь вроде как ничего сделать нельзя... или можно?
			log.Errorf("465 ERR_YOUREBANNEDCREEP, %s", e.Raw)
		})
		ircClient.AddCallback("471", func(e *irc.Event) {
			// Это значит, что народу на канале максимальное количество. Будем пробовать присунуться раз в 30 сек.
			// Джентельменам верим на слово и не проверям, должны ли мы джойниться к указанному каналу.
			log.Warnf("471 ERR_CHANNELISFULL, %s", e.Raw)
			time.Sleep(30 * time.Second)
			ircClient.Join(e.Arguments[1])
		})
		ircClient.AddCallback("472", func(e *irc.Event) {
			log.Errorf("472 ERR_UNKNOWNMODE, %s", e.Raw)
		})
		ircClient.AddCallback("473", func(e *irc.Event) {
			// Частенько такую хуйню творят, если надо "прибраться" либо в канале, либо на сервере.
			// По завершении работ +i снимают.
			log.Errorf("473 ERR_INVITEONLYCHAN, %s", e.Raw)
			time.Sleep(30 * time.Second)
			ircClient.Join(e.Arguments[1])
		})
		ircClient.AddCallback("474", func(e *irc.Event) {
			log.Errorf("474 ERR_BANNEDFROMCHAN, %s", e.Raw)
			time.Sleep(30 * time.Second)
			// Вдруг, нас забанили, но какбэ не навсегда?
			// N.B. Наверно, стОит проверять должен ли я быть заджоенным в этот канал?
			//      Но мы *верим* серверу на слово и подразумеваем, что были.
			ircClient.Join(e.Arguments[1])
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

		// Сделаем уже что-то полезное!
		ircClient.AddCallback("001", func(e *irc.Event) {
			if !config.Irc.Sasl && config.Irc.Password != "" && !nickIsUsed {
				log.Info("Identifying via NickServ")
				imChan <- iMsg{ChatId: "NickServ", Text: fmt.Sprintf("identify %s %s", config.Irc.Nick, config.Irc.Password)}
			}

			log.Debug("Trying to join to preconfigured channels")

			for _, channel := range config.Irc.Channels {
				log.Infof("Joining to %s channel", channel)
				ircClient.Join(channel)
			}

			// TODO: try to join to ivited cannels
		})

		ircClient.AddCallback("KICK", func(e *irc.Event) {
			if e.Arguments[1] == ircClient.GetNick() {
				// TODO: reason?
				log.Warnf("%s kicks us from %s", e.Source, e.Arguments[0])
				time.Sleep(10 * time.Second)
				// TODO: if someone invite us to channel, and any of ops are not approve bot - omit re-join on kick
				// TODO: if bot is not approved, remove self from invite list on kick!
				ircClient.Join(e.Arguments[0])
			} else {
				log.Infof("On %s %s kicks %s from channel", e.Arguments[0], e.Source, e.Arguments[1])
			}
		})

		ircClient.AddCallback("NICK", func(e *irc.Event) {
			if e.Nick == config.Irc.Nick {
				nickIsUsed = false

				if config.Irc.Password != "" {
					log.Warn("I regain my nick")
				} else {
					log.Warn("I regain my nick, trying to identify myself via NickServ")
					imChan <- iMsg{ChatId: "NickServ", Text: fmt.Sprintf("identify %s %s", config.Irc.Nick, config.Irc.Password)}
				}
			} else {
				log.Infof("%s renames themself to %s", e.Nick, e.Arguments[0])
			}
		})

		ircClient.AddCallback("JOIN", func(e *irc.Event) {
			if e.Nick == ircClient.GetNick() {
				log.Infof("I joined to %s", e.Arguments[0])
			} else {
				log.Infof("%s joined to %s", e.Source, e.Arguments[0])
			}
		})

		ircClient.AddCallback("PART", func(e *irc.Event) {
			if e.Nick == ircClient.GetNick() {
				log.Infof("I parted from %s", e.Arguments[0])
			} else {
				log.Infof("%s parted from %s", e.Source, e.Arguments[0])
			}
		})

		ircClient.AddCallback("QUIT", func(e *irc.Event) {
			// TODO: Quit message? But who really cares?
			if e.Nick == ircClient.GetNick() {
				log.Infof("I quit from %s", e.Arguments[0])
			} else {
				log.Infof("%s quit from %s", e.Source, e.Arguments[0])
			}
		})

		// TODO: парсить собственные режимы, на предмет войса и мьюта
		ircClient.AddCallback("MODE", func(e *irc.Event) {
			if e.Nick == ircClient.GetNick() {
				log.Infof("I set mode %s on %s", e.Arguments[1], e.Arguments[0])
			} else {
				log.Infof("%s set mode %s on %s", e.Source, e.Arguments[1], e.Arguments[0])
			}
		})

		ircClient.AddCallback("TOPIC", func(e *irc.Event) {
			if e.Nick == ircClient.GetNick() {
				log.Infof("I set topic on %s to %s", e.Arguments[0], e.Arguments[1])
			} else {
				log.Infof("%s set topic on %s to %s", e.Source, e.Arguments[0], e.Arguments[1])
			}
		})

		// TODO: Invite list should be somewhere saved
		// TODO: "Global ability to Invite" setting must be regulated by bot owner
		// TODO: bot must be approved by any of chan ops, so it can rejoin to channel if kicked
		ircClient.AddCallback("INVITE", func(e *irc.Event) {
			if e.Nick == ircClient.GetNick() {
				log.Infof("I invite %s to %s", e.Arguments[0], e.Arguments[1])
			} else {
				log.Infof("%s invites me to %s", e.Arguments[0], e.Arguments[1])
				// Наивно попробуем проследовать на тот канал, куда нас пригласили.
				ircClient.Join(e.Arguments[1])
			}
		})

		// TODO: Implement standard action like slap, f.ex.

		// Здесь у нас парсер сообщений из IRC
		ircClient.AddCallback("PRIVMSG", func(e *irc.Event) {
			log.Debugf("Incoming PRIVMSG: %s", spew.Sdump(e))
			ircMsgParser(e.Arguments[0], e.Nick, e.User, e.Source, e.Arguments[1])
		})

		log.Debugf("Make an actual connection to irc")

		if err := ircClient.Connect(serverString); err != nil {
			log.Fatalf("Unable to prepare irc connection: %s", err)
			time.Sleep(3 * time.Second)
			continue
		}

		ircClient.Loop()
	}
}

// ircMsgParser парсит сообщения, прилетевшие из IRC-ки
func ircMsgParser(channel string, nick string, user string, source string, msg string) {
	// nick - это выбранный пользователем nick (если он занят, то его "нарисует" сервер)
	// user - это короткое имя пользователя, под которым его видит сервер
	// source - это длинное имя пользователя, оно содержит в себе помимо user, ещё и ip с которого пришёл пользователь
	if shutdown {
		// Если мы завершаем работу программы, то нам ничего обрабатывать не надо
		return
	}

	switch {
	case msg == fmt.Sprintf("%shelp", config.Csign) || msg == fmt.Sprintf("%sпомощь", config.Csign):
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%shelp | %sпомощь             - это сообщение", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sanek | %sанек | %sанекдот    - рандомный анекдот с anekdot.ru", config.Csign, config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sbuni                       - комикс-стрип hapi buni", config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sbunny                      - кролик", config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%srabbit | %sкролик           - кролик", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%scat | %sкис                 - кошечка", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sdice | %sroll | %sкости      - бросить кости", config.Csign, config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sdig | %sкопать              - заняться археологией", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sdrink | %sпраздник          - какой сегодня праздник?", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sfish | %sfisher             - порыбачить", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sрыба | %sрыбка | %sрыбалка   - порыбачить", config.Csign, config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sf | %sф                     - рандомная фраза из сборника цитат fortune_mod", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sfortune | %sфортунка        - рандомная фраза из сборника цитат fortune_mod", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sfox | %sлис                 - лисичка", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sfriday | %sпятница          - а не пятница ли сегодня?", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sfrog | %sлягушка            - лягушка", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%shorse | %sлошадь | %sлошадка - лошадка", config.Csign, config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%skarma фраза                - посмотреть карму фразы", config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sкарма фраза                - посмотреть карму фразы", config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintln("фраза++ | фраза--           - повысить или понизить карму фразы")}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%slat | %sлат                 - сгенерировать фразу из крылатого латинского выражения", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%smonkeyuser                 - комикс-стрип MonkeyUser", config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sowl | %sсова                - сова", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sping | %sпинг               - попинговать бота", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sproverb | %sпословица       - рандомная русская пословица", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%ssnail | %sулитка            - улитка", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%ssome_brew                  - выдать соответствующий напиток, бармен может налить rum, ром, vodka, водку, tequila, текила, whisky, виски, absinthe, абсент", config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sver | %sversion             - написать что-то про версию ПО", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sверсия                     - написать что-то про версию ПО", config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sw город | %sп город         - погода в городе", config.Csign, config.Csign)}
		imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sxkcd                       - комикс-стрип с xkcb.ru", config.Csign)}
		// TODO: команда get lost, чтобы бот свалил с канальчика, куда его поинвайтили и удалил его из invite list
	default:
		var message sMsg
		message.From = config.Redis.MyChannel
		message.Userid = user    // как его видит сервер
		message.Chatid = channel // чятик, в который написал user
		message.Message = msg
		message.Plugin = config.Redis.MyChannel

		// Если channel - это ник бота, то беседа приватная
		if channel == ircClient.GetNick() {
			message.Mode = "private"
			message.Chatid = user // так проще ориентироваться
		} else {
			message.Mode = "public"
		}

		message.Misc.Fwdcnt = 0
		message.Misc.Csign = config.Csign
		message.Misc.Username = nick
		message.Misc.Answer = 1
		message.Misc.Botnick = config.Irc.Nick
		message.Misc.Msgformat = 0

		data, err := json.Marshal(message)

		if err != nil {
			log.Warnf("Unable to to serialize message for redis: %s", err)
			return
		}

		// Заталкиваем наш json в редиску
		if err := redisClient.Publish(ctx, config.Redis.Channel, data).Err(); err != nil {
			log.Warnf("Unable to send data to redis channel %s: %s", config.Redis.Channel, err)
		} else {
			log.Debugf("Send msg to redis channel %s: %s", config.Redis.Channel, string(data))
		}

		return
	}
}

// redisMsgParser парсит json-чики прилетевшие из REDIS-ки, причём, json-чики должны быть относительно валидными
func redisMsgParser(msg string) {
	if shutdown {
		// Если мы завершаем работу программы, то нам ничего обрабатывать не надо
		return
	}

	var j rMsg

	log.Debugf("Incoming raw json: %s", msg)

	if err := json.Unmarshal([]byte(msg), &j); err != nil {
		log.Warnf("Unable to to parse message from redis channel: %s", err)
		return
	}

	// Validate our j
	if exist := j.From; exist == "" {
		log.Warnf("Incorrect msg from redis, no from field: %s", msg)
		return
	}

	if exist := j.Chatid; exist == "" {
		log.Warnf("Incorrect msg from redis, no chatid field: %s", msg)
		return
	}

	if exist := j.Userid; exist == "" {
		log.Warnf("Incorrect msg from redis, no userid field: %s", msg)
		return
	}

	if exist := j.Message; exist == "" {
		log.Warnf("Incorrect msg from redis, no message field: %s", msg)
		return
	}

	if exist := j.Plugin; exist == "" {
		log.Warnf("Incorrect msg from redis, no plugin field: %s", msg)
		return
	}

	if exist := j.Mode; exist == "" {
		log.Warnf("Incorrect msg from redis, no mode field: %s", msg)
		return
	}

	// j.Misc.Answer может и не быть, тогда ответа на такое сообщение не будет
	if j.Misc.Answer == 0 {
		log.Debug("Field Misc->Answer = 0, skipping message")
		return
	}

	// j.Misc.BotNick тоже можно не передавать, тогда будет записана пустая строка
	// j.Misc.CSign если нам его не передали, возьмём значение из конфига
	if exist := j.Misc.Csign; exist == "" {
		j.Misc.Csign = config.Csign
	}

	// j.Misc.FwdCnt если нам его не передали, то будет 0
	if exist := j.Misc.Fwdcnt; exist == 0 {
		j.Misc.Fwdcnt = 1
	}

	// j.Misc.GoodMorning может быть быть 1 или 0, по-умолчанию 0
	// j.Misc.MsgFormat может быть быть 1 или 0, по-умолчанию 0
	// j.Misc.Username можно не передавать, тогда будет пустая строка

	// Отвалидировались, теперь вернёмся к нашим баранам.
	imChan <- iMsg{ChatId: j.Chatid, Text: j.Message}

	return
}

// ircSend отправляет сообщение в irc. Якобы с применением ratelimit-ов, но на практике длинные сообщения разделяются
// на более короткие и тут ratelimit не срабатывает. Если не повезёт, то сервер может за такое дело и кикнуть.
func ircSend() {
	for {
		m := <-imChan
		ircClient.Privmsg(m.ChatId, m.Text)
		time.Sleep(time.Duration(config.Irc.Delay) * time.Millisecond)
	}
}
