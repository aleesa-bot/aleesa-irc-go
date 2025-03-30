package main

import (
	"crypto/tls"
	"fmt"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	irc "github.com/thoj/go-ircevent"
)

// ircClientInit горутинка для работы с протоколом irc.
func ircClientRun() {
	for !shutdown {
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

		// Навесим коллбэков на некоторые ответы сервера на наши запросы.

		// 001 RPL_WELCOME уже есть в github.com/thoj/go-ircevent/irc_callback.go.
		// Сделаем уже что-то полезное! Motd нам уже прислали и теперь можно авторизоваться и джойниться
		ircClient.AddCallback("004", func(e *irc.Event) {
			// Из это строки мы можем узнать, какие флаги для user MODE и channem MODE можно навешивать.
			// Как минимум, эта строка нам нужна, чтобы выяснить, можем ли мы взять себе +B, мы же бот :).
			log.Debugf("004 RPL_MYINFO, %s", e.Raw)

			e.Connection.Lock()

			// Формат строки с MODE-ами https://datatracker.ietf.org/doc/html/rfc2812#section-5.1 .
			log.Debugf("Add %s to list of available user modes", e.Arguments[3])

			for _, mode := range strings.Split(e.Arguments[3], "") {
				availableUserModes.Set(mode, true)
			}

			log.Debugf("Add %s to list available channel modes", e.Arguments[4])

			for _, mode := range strings.Split(e.Arguments[4], "") {
				availableChanModes.Set(mode, true)
			}

			e.Connection.Unlock()
		})

		ircClient.AddCallback("319", func(e *irc.Event) {
			/* Это одна из строк с данными, прилетающая в ответ на запрос whois на определённого юзера
			 * Из этой строки нас интересует, на каких каналах пользователь op (то есть с префиксом @) или имеет voice
			 * (то есть с префиксом +) чтобы внести его в свою базу mode-ов.
			 */
			log.Debugf("319 RPL_WHOISCHANNELS, %s", e.Raw)
			channelsWithModes := strings.Split(e.Arguments[2], " ")
			dstNick := e.Arguments[1]

			for _, channelWithMode := range channelsWithModes {
				for _, mychannel := range config.Irc.Channels {
					tmp := regexp.MustCompile("#").Split(channelWithMode, 2)
					channel := tmp[1]
					modes := tmp[0]

					if channel == mychannel {
						// Для каждого канала формат строго 1 из 4-х:  #channel | @#channel | +#channel | @+#channel
						switch modes {
						case "@+":
							userModeUpdateUser(channel, dstNick, "+o")
							userModeUpdateUser(channel, dstNick, "+v")
						case "@":
							userModeUpdateUser(channel, dstNick, "+o")
						case "+":
							userModeUpdateUser(channel, dstNick, "+v")
						default:
							userModeUpdateUser(channel, dstNick, "-o")
							userModeUpdateUser(channel, dstNick, "-v")
						}

						break
					}
				}
			}
		})

		// 433 ERR_NICKNAMEINUSE уже есть в github.com/thoj/go-ircevent/irc_callback.go.

		ircClient.AddCallback("353", func(e *irc.Event) {
			// Это одна из строк данных, прилетающая в ответ на запрос names, на канале
			log.Debugf("353 RPL_NAMREPLY, %s", e.Raw)
			e.Connection.Lock()
			namesString := e.Arguments[3]
			channel := e.Arguments[2]

			for _, mychannel := range config.Irc.Channels {
				if channel == mychannel {
					for _, name := range strings.Split(namesString, " ") {
						mode := name[:1]

						var nick string

						if mode == "@" || mode == "+" {
							nick = name[1:]
						} else {
							nick = name
						}

						switch mode {
						case "@":
							userModeUpdateUser(channel, nick, "+o")
						case "+":
							userModeUpdateUser(channel, nick, "+v")
						default:
							userModeUpdateUser(channel, nick, "-o")
							userModeUpdateUser(channel, nick, "-v")
						}
					}

					break
				}
			}
			e.Connection.Unlock()
		})

		ircClient.AddCallback("352", func(e *irc.Event) {
			// Относительно бесполезная для нас команда. Не возвращает канала (в случае с ngircd), так что узнать mode
			// юзера на конкретном канале из этого невозможно. А остальные данные пока непонятно куда применять.
			log.Warnf("352 RPL_WHOREPLY, %s", e.Raw)
		})

		// Если сервер не может прочитать MOTD, то он может вернуть 422 ERR_NOMOTD, тоде самое навесим и туда тоже.
		ircClient.AddCallback("376", func(e *irc.Event) {
			log.Debugf("376 RPL_ENDOFMOTD %s", e.Raw)

			// Если у нас есть доступный +B возьмём его себе, мы же бот.
			botFlag, ok := availableUserModes.Get("B")

			if ok && botFlag {
				log.Info("Grabbing +B flag to mark me as bot")
				ircClient.Mode(ircClient.GetNick(), "+B")
			} else {
				if ok {
					log.Info("Skip +B flag, server does not support it, noone will know that I am bot")
				} else {
					log.Info("Skip +B flag, server did not announce modes (yet?), noone will know that I am bot")
				}
			}

			if !config.Irc.Sasl && config.Irc.Password != "" && !nickIsUsed {
				log.Info("Identifying via NickServ")

				message := fmt.Sprintf("identify %s %s", config.Irc.Nick, config.Irc.Password)
				imChan <- iMsg{ChatID: "NickServ", Text: message}
			}
			// TODO: wait until +R flag been set? could be implemented with waiting for channel message.

			log.Debug("Trying to join to preconfigured channels")

			for _, channel := range config.Irc.Channels {
				log.Infof("Joining to %s channel", channel)
				ircClient.Join(channel)
			}
		})

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

		// Аналогичный коллбэк висит на 376 RPL_ENDOFMOTD.
		ircClient.AddCallback("422", func(e *irc.Event) {
			log.Debugf("422 ERR_NOMOTD %s", e.Raw)

			// Если у нас есть доступный +B возьмём его себе, мы же бот.
			botFlag, ok := availableUserModes.Get("B")

			if ok && botFlag {
				log.Info("Grabbing +B flag to mark me as bot")
				ircClient.Mode(ircClient.GetNick(), "+B")
			} else {
				if ok {
					log.Info("Skip +B flag, server does not support it, noone will know that I am bot")
				} else {
					log.Info("Skip +B flag, server did not announce modes (yet?), noone will know that I am bot")
				}
			}

			if !config.Irc.Sasl && config.Irc.Password != "" && !nickIsUsed {
				log.Info("Identifying via NickServ")

				message := fmt.Sprintf("identify %s %s", config.Irc.Nick, config.Irc.Password)
				imChan <- iMsg{ChatID: "NickServ", Text: message}
			}
			// TODO: wait until +R flag been set? could be implemented with waiting for channel message.

			log.Debug("Trying to join to preconfigured channels")

			for _, channel := range config.Irc.Channels {
				log.Infof("Joining to %s channel", channel)
				ircClient.Join(channel)
			}
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

			<-time.NewTimer(60 * time.Second).C // 1 minute sleep.

			ircClient.Nick(config.Irc.Nick)
		})

		ircClient.AddCallback("436", func(e *irc.Event) {
			// Что это за зверь такой?
			// Предположительно, тут имеется в виду ситуация, когда в конфедерации серверов ник был зареган на двух
			// серверах и теперь сервер не знает, что с этим делать
			log.Errorf("436 ERR_NICKCOLLISION, %s", e.Raw)
		})

		// 437: ERR_UNAVAILRESOURCE уже есть в github.com/thoj/go-ircevent/irc_callback.go.
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
			channel := e.Arguments[1]
			log.Warnf("471 ERR_CHANNELISFULL, %s", e.Raw)
			time.Sleep(30 * time.Second)

			// Проверяем, а должны ли мы быть заджоенныеми к указанному, каналу, а то вдруг нет?
			for _, ircChan := range config.Irc.Channels {
				if ircChan == channel {
					ircClient.Join(channel)

					break
				}
			}
		})

		ircClient.AddCallback("472", func(e *irc.Event) {
			log.Errorf("472 ERR_UNKNOWNMODE, %s", e.Raw)
		})

		ircClient.AddCallback("473", func(e *irc.Event) {
			// Частенько такую хуйню творят, если надо "прибраться" либо в канале, либо на сервере.
			// По завершении работ +i снимают.
			channel := e.Arguments[1]
			log.Errorf("473 ERR_INVITEONLYCHAN, %s", e.Raw)
			time.Sleep(30 * time.Second)

			// Проверяем, а должны ли мы быть заджоенныеми к указанному, каналу, а то вдруг нет?
			for _, ircChan := range config.Irc.Channels {
				if ircChan == channel {
					ircClient.Join(channel)

					break
				}
			}
		})

		ircClient.AddCallback("474", func(e *irc.Event) {
			// Это событие прилетает, (только) если мы пытаемся приджойниться к каналу, где нас забанили
			channel := e.Arguments[1]

			log.Errorf("474 ERR_BANNEDFROMCHAN, %s", e.Raw)
			// Если нас забанили, то информацию о MODE-ах пользователей мы теряем
			time.Sleep(30 * time.Second)
			// Вдруг, нас забанили, но какбэ не навсегда?

			// Проверяем, а должны ли мы быть заджоенныеми к указанному, каналу, а то вдруг нет?
			for _, ircChan := range config.Irc.Channels {
				if ircChan == channel {
					ircClient.Join(channel)

					break
				}
			}
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

		// Навесим коллбэков на другие, интересные нам события
		ircClient.AddCallback("KICK", func(e *irc.Event) {
			dstNick := e.Arguments[1]
			srcFullNick := e.Source
			channel := e.Arguments[0]

			if e.Arguments[1] == ircClient.GetNick() {
				// Нас кикнули с канала, и мы теряем информацию о MODE-ах пользователей
				userMode.Delete(channel)
				// TODO: reason?
				log.Warnf("%s kicks us from %s", srcFullNick, channel)
				time.Sleep(10 * time.Second)
				ircClient.Join(channel)
			} else {
				// Кого-то другого кикнули с канала
				log.Infof("On %s %s kicks %s from channel", channel, srcFullNick, dstNick)
				userModeDeleteUser(channel, dstNick)
			}
		})

		ircClient.AddCallback("NICK", func(e *irc.Event) {
			srcNick := e.Nick
			dstNick := e.Arguments[0]

			if e.Nick == config.Irc.Nick {
				nickIsUsed = false

				if config.Irc.Password != "" {
					log.Warn("I regain my nick")
				} else {
					log.Warn("I regain my nick, trying to identify myself via NickServ")

					message := fmt.Sprintf("identify %s %s", config.Irc.Nick, config.Irc.Password)
					imChan <- iMsg{ChatID: "NickServ", Text: message}
				}
			} else {
				log.Infof("%s renames themself to %s", srcNick, dstNick)
			}

			// Неважно чей ник сменился, надо забыть, что было и снова узнать mode-ы сменишего nick джентельмена
			// TODO: реализовать userModeRenameUser()
			userModePurgeUser(srcNick)
			ircClient.Whois(dstNick)
		})

		ircClient.AddCallback("JOIN", func(e *irc.Event) {
			nick := e.Nick
			fullNick := e.Source
			channel := e.Arguments[0]

			if nick == ircClient.GetNick() {
				log.Infof("I joined to %s", channel)
				// Команда names отправляется автоматом
			} else {
				log.Infof("%s joined to %s", fullNick, channel)
				// Технически, тут не надо спрашивать whois на пользователя, но мы спрашиваем, чтобы уточнить mode,
				// вдруг сервер проставляет mode заранее (хотя не должен)
				ircClient.Whois(nick)
			}
		})

		ircClient.AddCallback("PART", func(e *irc.Event) {
			nick := e.Nick
			fullNick := e.Source
			channel := e.Arguments[0]

			if nick == ircClient.GetNick() {
				log.Infof("I parted from %s", channel)
				userMode.Delete(channel)
			} else {
				log.Infof("%s parted from %s", fullNick, channel)
				userModeDeleteUser(channel, nick)
			}
		})

		ircClient.AddCallback("QUIT", func(e *irc.Event) {
			nick := e.Nick
			fullNick := e.Source

			// TODO: Quit message? But who really cares?
			if nick == ircClient.GetNick() {
				log.Info("I quit")
			} else {
				log.Infof("%s has quit", fullNick)
				// Товарищ свалил из irc, забудем про его mode-ы
				userModePurgeUser(nick)
			}
		})

		ircClient.AddCallback("MODE", func(e *irc.Event) {
			mode := e.Arguments[1]
			channel := e.Arguments[0]
			fullSrcNick := e.Source
			srcNick := e.Nick

			var dstNick string

			if len(e.Arguments) >= 3 { // кого-то по-MODE-или на канале
				dstNick = e.Arguments[2]

				if srcNick == ircClient.GetNick() {
					log.Infof("I set user %s mode %s on %s", dstNick, mode, channel)
				} else {
					log.Infof("%s set user %s mode %s on %s", fullSrcNick, dstNick, mode, channel)
				}
			} else { // Установка mode-а при заходе на сервер
				dstNick = e.Arguments[0]

				log.Infof("Server set my mode to %s", mode)
			}

			userModeUpdateUser(channel, dstNick, mode)
		})

		ircClient.AddCallback("TOPIC", func(e *irc.Event) {
			nick := e.Nick
			fullNick := e.Source
			topic := e.Arguments[1]
			channel := e.Arguments[0]

			if nick == ircClient.GetNick() {
				log.Infof("I set topic on %s to %s", channel, topic)
			} else {
				log.Infof("%s set topic on %s to %s", fullNick, channel, topic)
			}
		})

		ircClient.AddCallback("INVITE", func(e *irc.Event) {
			srcNick := e.Nick
			dstNick := e.Arguments[0]
			channel := e.Arguments[1]

			if srcNick == ircClient.GetNick() {
				log.Infof("I invite %s to %s", dstNick, channel)
			} else {
				log.Infof("%s invites me to %s", srcNick, channel)
			}
		})

		// TODO: Implement standard action like slap, f.ex.

		// Здесь у нас парсер сообщений из IRC
		ircClient.AddCallback("PRIVMSG", func(e *irc.Event) {
			log.Debugf("Incoming PRIVMSG: %s", e.Raw)
			ircMsgParser(e.Arguments[0], e.Nick, e.User, e.Source, e.Arguments[1])
		})

		/*
			ircClient.AddCallback("*", func(e *irc.Event) {
				log.Debugf("Incoming EVENT (Raw): %s", e.Raw)
			})
		*/

		log.Debugf("Make an actual connection to irc")

		if err := ircClient.Connect(serverString); err != nil {
			log.Errorf("Unable to prepare irc connection: %s", err)
			<-time.NewTimer(3 * time.Second).C // Sleep for 3 seconds.

			continue
		}

		ircClient.Loop()
	}
}

// ircSend отправляет сообщение в irc. Якобы с применением ratelimit-ов, но на практике длинные сообщения разделяются
// на более короткие и тут ratelimit не срабатывает. Если не повезёт, то сервер может за такое дело и кикнуть.
func ircSend() {
	for {
		m := <-imChan

		switch config.Irc.RateLimit.Type {
		case "simple_delay":
			log.Debugf("Sending to chat %s message: %s", m.ChatID, m.Text)

			if m.Text[0:4] == "/me " {
				ircClient.Action(m.ChatID, m.Text)
			} else {
				ircClient.Privmsg(m.ChatID, m.Text)
			}

			sleepDelay := time.Duration(config.Irc.RateLimit.SimpleDelay) * time.Millisecond
			log.Debugf("Due to delay type simple_delay waiting for %d milliseconds", int(sleepDelay))
			time.Sleep(sleepDelay)
		case "token_bucket":
			var currentTimeMs = time.Now().UnixMilli()

			var expirationTimeMs = config.Irc.RateLimit.TokenBucket.ExpirationTime * 1000

			if len(msgBucket.Timestamps) > 0 {
				var newBucket bucket

				for _, timestampMs := range msgBucket.Timestamps {
					if currentTimeMs-timestampMs < expirationTimeMs {
						newBucket.Timestamps = append(newBucket.Timestamps, timestampMs)
					}
				}

				bucketFill := len(newBucket.Timestamps)
				log.Debugf("Message bucket filled with %d/%d messages", bucketFill, config.Irc.RateLimit.TokenBucket.Size)

				newBucket.Timestamps = append(newBucket.Timestamps, currentTimeMs)

				if bucketFill >= config.Irc.RateLimit.TokenBucket.Size {
					newBucket.IsFull = true
				}

				msgBucket = newBucket
			} else {
				// Проставим время отправки сообщения
				log.Debug("Message bucket is empty")

				msgBucket.Timestamps = append(msgBucket.Timestamps, currentTimeMs)
			}

			if msgBucket.IsFull {
				log.Debug("Message bucket overflowed, hitting ratelimit")

				sleepPeriod := expirationTimeMs - (currentTimeMs - msgBucket.Timestamps[len(msgBucket.Timestamps)-1])

				if config.Irc.RateLimit.TokenBucket.Limit > 0 {
					sleepPeriod /= int64(config.Irc.RateLimit.TokenBucket.Limit)
				}

				log.Debugf("Sleeping for %d milliseconds", sleepPeriod)
				time.Sleep(time.Duration(sleepPeriod) * time.Millisecond)
				currentTimeMs = time.Now().UnixMilli()
				// Обновим время отправки сообщения
				msgBucket.Timestamps[len(msgBucket.Timestamps)-1] = currentTimeMs
			}

			log.Debugf("Sending to chat %s message: %s", m.ChatID, m.Text)

			if len(m.Text) > 4 && m.Text[0:4] == "/me " {
				ircClient.Action(m.ChatID, m.Text[4:])
			} else {
				ircClient.Privmsg(m.ChatID, m.Text)
			}
		default:
			log.Debugf("Sending to chat %s message: %s", m.ChatID, m.Text)

			if len(m.Text) > 4 && m.Text[0:4] == "/me " {
				ircClient.Action(m.ChatID, m.Text[4:])
			} else {
				ircClient.Privmsg(m.ChatID, m.Text)
			}
		}
	}
}

func ircSendUnrestricted() {
	for {
		m := <-imChanUnrestricted
		log.Debugf("Skipping ratelimit and sending to chat %s message: %s", m.ChatID, m.Text)

		if len(m.Text) > 4 && m.Text[0:4] == "/me " {
			ircClient.Action(m.ChatID, m.Text[4:])
		} else {
			ircClient.Privmsg(m.ChatID, m.Text)
		}
	}
}

/* vim: set ft=go noet ai ts=4 sw=4 sts=4: */
