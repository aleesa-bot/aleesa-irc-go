package main

import (
	"encoding/json"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
)

// redisMsgParser парсит json-чики прилетевшие из REDIS-ки
func redisMsgParser(msg string) {
	var sendTo string
	var j rMsg

	log.Debugf("Incomming raw json: %s", msg)

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
	} else {
		sendTo = j.Plugin
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

	log.Error(spew.Sdump(j))
	log.Debug(sendTo)
	return
}

// ircMsgParser парсит сообщения, прилетевшие из IRC-ки
func ircMsgParser(channel string, nick string, source string, msg string) {
	switch {
	case msg == fmt.Sprintf("%shelp", config.Csign) || msg == fmt.Sprintf("%sпомощь", config.Csign):
		ircClient.Privmsg(nick, fmt.Sprintf("%shelp | %sпомощь             - это сообщение", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sanek | %sанек | %sанекдот    - рандомный анекдот с anekdot.ru", config.Csign, config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sbuni                       - комикс-стрип hapi buni", config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sbunny                      - кролик", config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%srabbit | %sкролик           - кролик", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%scat | %sкис                 - кошечка", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sdice | %sroll | %sкости      - бросить кости", config.Csign, config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sdig | %sкопать              - заняться археологией", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sdrink | %sпраздник          - какой сегодня праздник?", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sfish | %sfisher             - порыбачить", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sрыба | %sрыбка | %sрыбалка   - порыбачить", config.Csign, config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sf | %sф                     - рандомная фраза из сборника цитат fortune_mod", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sfortune | %sфортунка        - рандомная фраза из сборника цитат fortune_mod", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sfox | %sлис                 - лисичка", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sfriday | %sпятница          - а не пятница ли сегодня?", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sfrog | %sлягушка            - лягушка", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%shorse | %sлошадь | %sлошадка - лошадка", config.Csign, config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%skarma фраза                - посмотреть карму фразы", config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sкарма фраза                - посмотреть карму фразы", config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintln("фраза++ | фраза--           - повысить или понизить карму фразы"))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%slat | %sлат                 - сгенерировать фразу из крылатого латинского выражения", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%smonkeyuser                 - комикс-стрип MonkeyUser", config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sowl | %sсова                - сова", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sping | %sпинг               - попинговать бота", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sproverb | %sпословица       - рандомная русская пословица", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%ssnail | %sулитка            - улитка", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%ssome_brew                  - выдать соответствующий напиток, бармен может налить rum, ром, vodka, водку, beer, пиво, tequila, текила, whisky, виски, absinthe, абсент", config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sver | %sversion             - написать что-то про версию ПО", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sверсия                     - написать что-то про версию ПО", config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sw город | %sп город         - погода в городе", config.Csign, config.Csign))
		time.Sleep(250 * time.Millisecond)
		ircClient.Privmsg(nick, fmt.Sprintf("%sxkcd                       - комикс-стрип с xkcb.ru", config.Csign))

	default:
		var message sMsg
		message.From = config.Redis.MyChannel
		message.Userid = nick
		message.Chatid = channel
		message.Message = msg
		message.Plugin = config.Redis.MyChannel
		message.Mode = "public"
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

		// Редиска не нужна, если мы можем передать это всё прямо в функцию-парсер сообщений.

		redisMsgParser(string(data))
		return
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
	log.Error("Starting IRC client")
	go ircClientRun()

	// Самое время поставить траппер сигналов
	signal.Notify(sigChan,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	log.Error("starting signal handler")
	go sigHandler()

	log.Error("running sleep-loop")
	for {
		time.Sleep(1 * time.Hour)
	}
}
