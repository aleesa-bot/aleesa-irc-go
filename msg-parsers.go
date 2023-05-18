package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// ircMsgParser парсит сообщения, прилетевшие из IRC-ки
func ircMsgParser(channel string, nick string, user string, source string, msg string) {
	// nick - это выбранный пользователем nick (если он занят, то его "нарисует" сервер)
	// user - это короткое имя пользователя, под которым его видит сервер
	// source - это длинное имя пользователя, оно содержит в себе помимо user, ещё и ip с которого пришёл пользователь
	if shutdown {
		// Если мы завершаем работу программы, то нам ничего обрабатывать не надо
		return
	}

	if channel == ircClient.GetNick() {
		// В привате бот не отвечает, чтобы не было возможности DDoS-а, ratelimit-ы в irc слишком жёсткие
		// TODO: возможно, это имеет смысл вынести в конфиг, но это если кому-то кроме меня бот будет интересен
		return
	}

	// Ловим команды и обрабатываем их
	if (len(msg) > len(config.Csign)) && (msg[:len(config.Csign)] == config.Csign) {
		var outgoingMessage string

		var message sMsg
		message.From = config.Redis.MyChannel
		message.Userid = user    // как его видит сервер
		message.Chatid = channel // чятик, в который написал user
		message.Threadid = ""    // тредиков в irc нету, поэтому это поле отправляем пустым
		message.Plugin = config.Redis.MyChannel
		message.Mode = "public"
		// Предполагается, что на команды бот автоматом отвечает
		message.Misc.Answer = 1
		message.Misc.Fwdcnt = 0
		message.Misc.Csign = config.Csign
		message.Misc.Username = nick
		message.Misc.Botnick = config.Irc.Nick
		message.Misc.Msgformat = 0

		var cmd = msg[len(config.Csign):]

		switch {
		case cmd == "help" || msg == "помощь":
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

			if userModeIsOped(channel, nick) {
				imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sadmin                      - настройки некоторых плагинов бота для канала", config.Csign)}
			}

			return

		case cmd == "admin":
			if userModeIsOped(channel, nick) {
				imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sadmin oboobs #        - где 1 - вкл, 0 - выкл плагина oboobs", config.Csign)}
				imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sadmin oboobs         показываем ли сисечки по просьбе участников чата (команды %stits, %stities, %sboobs, %sboobies, %sсиси, %sсисечки)", config.Csign, config.Csign, config.Csign, config.Csign, config.Csign, config.Csign, config.Csign)}
				imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sadmin obutts #        - где 1 - вкл, 0 - выкл плагина obutts", config.Csign)}
				imChan <- iMsg{ChatId: nick, Text: fmt.Sprintf("%sadmin obutts         показываем ли попки по просьбе участников чата (команды %sass, %sbutt, %sbooty, %sпопа, %sпопка)", config.Csign, config.Csign, config.Csign, config.Csign, config.Csign, config.Csign)}
			}

			return

		case cmd == "admin oboobs":
			if userModeIsOped(channel, nick) {
				value := getSetting(channel, "oboobs")

				switch {
				case value == "":
					_ = saveSetting(channel, "oboobs", "0")
					imChan <- iMsg{ChatId: nick, Text: "Плагин oboobs выключен"}
				case value == "0":
					imChan <- iMsg{ChatId: nick, Text: "Плагин oboobs выключен"}
				case value == "1":
					imChan <- iMsg{ChatId: nick, Text: "Плагин oboobs включен"}
				default:
					imChan <- iMsg{ChatId: nick, Text: "Плагин oboobs выключен"}
				}
			}

			return

		case cmd == "admin oboobs 1":
			if userModeIsOped(channel, nick) {
				err := saveSetting(channel, "oboobs", "1")

				if err != nil {
					imChan <- iMsg{ChatId: nick, Text: "Плагин oboobs всё ещё выключен"}
				} else {
					imChan <- iMsg{ChatId: nick, Text: "Плагин oboobs включен"}
				}
			}

			return

		case cmd == "admin oboobs 0":
			if userModeIsOped(channel, nick) {
				_ = saveSetting(channel, "oboobs", "0")
				imChan <- iMsg{ChatId: nick, Text: "Плагин oboobs выключен"}
			}

			return

		case cmd == "admin obutts":
			if userModeIsOped(channel, nick) {
				value := getSetting(channel, "obutts")

				switch {
				case value == "":
					_ = saveSetting(channel, "obutts", "0")
					imChan <- iMsg{ChatId: nick, Text: "Плагин obutts выключен"}
				case value == "0":
					imChan <- iMsg{ChatId: nick, Text: "Плагин obutts выключен"}
				case value == "1":
					imChan <- iMsg{ChatId: nick, Text: "Плагин obutts включен"}
				default:
					imChan <- iMsg{ChatId: nick, Text: "Плагин obutts выключен"}
				}
			}

			return

		case cmd == "admin obutts 1":
			if userModeIsOped(channel, nick) {
				err := saveSetting(channel, "obutts", "1")

				if err != nil {
					imChan <- iMsg{ChatId: nick, Text: "Плагин obutts всё ещё выключен"}
				} else {
					imChan <- iMsg{ChatId: nick, Text: "Плагин obutts включен"}
				}
			}

			return

		case cmd == "admin obutts 0":
			if userModeIsOped(channel, nick) {
				_ = saveSetting(channel, "obutts", "0")
				imChan <- iMsg{ChatId: nick, Text: "Плагин obutts выключен"}
			}

			return

		default:
			var done = false

			// Общий список простых команд
			cmds := []string{"ping", "пинг", "пинх", "pong", "понг", "понх", "coin", "монетка", "roll", "dice", "кости",
				"ver", "version", "версия", "хэлп", "halp", "kde", "кде", "lat", "лат", "friday", "пятница", "proverb",
				"пословица", "пословиться", "fortune", "фортунка", "f", "ф", "anek", "анек", "анекдот", "buni", "cat",
				"кис", "drink", "праздник", "fox", "лис", "frog", "лягушка", "horse", "лошадь", "лошадка", "monkeyuser",
				"owl", "сова", "сыч", "rabbit", "bunny", "кролик", "snail", "улитка", "xkcd", "dig", "копать", "fish",
				"fishing", "рыба", "рыбка", "рыбалка", "karma", "карма"}

			for _, command := range cmds {
				if cmd == command {
					done = true
					outgoingMessage = msg
					break
				}
			}

			// Список команд бармэна
			if !done {
				cmds := []string{"rum", "ром", "vodka", "водка", "beer", "пиво", "tequila", "текила", "whisky", "виски",
					"absinthe", "абсент"}

				// Тихо сам с собою я веду беседу...
				for _, command := range cmds {
					if cmd == command {
						done = true
						outgoingMessage = msg
						break
					}
				}

				// Заказываю выпивку кому-то ещё
				if !done {
					re := regexp.MustCompile(" +")
					pile := re.Split(cmd, 2)

					for _, command := range cmds {
						if pile[0] == command {
							userNick := strings.TrimSpace(pile[1])

							if userNick != "" {
								if userModeIsHere(channel, userNick) {
									// Проставляем правильный в конкретно данном случае username, так как отвечать мы
									// будем ему
									message.Misc.Username = strings.TrimSpace(pile[1])
								} else {
									msg = fmt.Sprintf("Я тут не вижу участника с ником %s", userNick)
									imChan <- iMsg{ChatId: channel, Text: msg}
									return
								}
							} // в противном случае юзер просто поставил пробел на конце команды, интерпретируем это как
							//   стаканчик юзеру

							outgoingMessage = msg
							done = true
							break
						}
					}
				}
			}

			// Сложные команды, например, погода
			if !done {
				cmdLen := len(cmd)

				cmds := []string{"w ", "п ", "погода ", "погодка ", "погадка ", "weather "}

				for _, command := range cmds {
					if cmdLen > len(command) && cmd[0:len(command)] == command {
						done = true
						outgoingMessage = msg
						break
					}
				}
			}

			// Отключаемые команды
			if !done {
				value := getSetting(channel, "obutts")

				if value == "1" {
					cmds := []string{"butt", "booty", "ass", "попа", "попка"}

					for _, command := range cmds {
						cmdLen := len(command)
						if cmd[:cmdLen] == command {
							done = true
							outgoingMessage = msg
							break
						}
					}
				}

				if !done {
					value = getSetting(channel, "oboobs")

					if value == "1" {
						cmds := []string{"tits", "boobs", "tities", "boobies", "сиси", "сисечки"}

						for _, command := range cmds {
							cmdLen := len(command)
							if cmd[:cmdLen] == command {
								// done = true
								outgoingMessage = msg
								break
							}
						}
					}
				}
			}
		}

		if outgoingMessage != "" {
			message.Message = outgoingMessage
			data, err := json.Marshal(message)

			if err != nil {
				log.Warnf("Unable to to serialize message for redis: %s", err)
				return
			}

			// Заталкиваем наш json в редиску
			if err := redisClient.Publish(ctx, config.Redis.Channel, data).Err(); err != nil {
				log.Warnf("Unable to send data to redis channel %s: %s", config.Redis.Channel, err)
			} else {
				log.Debugf("Sent msg to redis channel %s: %s", config.Redis.Channel, string(data))
			}
		}
	} else {
		// Это уже просто трёп в чятике
		var message sMsg
		message.From = config.Redis.MyChannel
		message.Userid = user    // как его видит сервер
		message.Chatid = channel // чятик, в который написал user
		message.Threadid = ""    // тредиков в irc нету, поэтому это поле отправляем пустым
		message.Message = msg
		message.Plugin = config.Redis.MyChannel
		message.Mode = "public"

		// Если во фразе содержится ключевые слова, под это есть костыль в craniac-е - (предполагаем, что) он
		// знает ставить или не ставить флажок answer
		message.Misc.Answer = 0

		// Предполагается что в канале бот должен отвечать, только если к нему обратились, либо это была команда
		if regexp.MustCompile(ircClient.GetNick()).Match([]byte(message.Message)) {
			message.Misc.Answer = 1
		}

		// Попробуем выискать изменение кармы
		msgLen := len(msg)

		// ++ или -- на конце фразы - это наш кандидат
		if msgLen > len("++") {
			if msg[msgLen-len("--"):msgLen] == "--" || msg[msgLen-len("++"):msgLen] == "++" {
				// Предполагается, что менять карму мы будем для одной фразы, то есть для 1 строки
				if len(strings.Split(msg, "\n")) == 1 {
					// Костыль для кармы
					message.Misc.Answer = 1
				}
			}
		}

		message.Misc.Fwdcnt = 0
		message.Misc.Csign = config.Csign
		message.Misc.Username = nick
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
			log.Debugf("Sent msg to redis channel %s: %s", config.Redis.Channel, string(data))
		}
	}

	return
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
	lines := regexp.MustCompile("\r?\n").Split(j.Message, -1)

	for _, message := range lines {
		if userModeIsOped(j.Chatid, ircClient.GetNick()) || userModeIsVoiced(j.Chatid, ircClient.GetNick()) {
			imChanUnrestricted <- iMsg{ChatId: j.Chatid, Text: message}
		} else {
			imChan <- iMsg{ChatId: j.Chatid, Text: message}
		}
	}

	return
}

/* vim: set ft=go noet ai ts=4 sw=4 sts=4: */
