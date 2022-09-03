package main

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
		Sasl      bool     `json:"sasl,omitempty"`
		Channels  []string `json:"channels"`
		RateLimit struct {
			Type        string `json:"type,omitempty"`
			SimpleDelay int    `json:"simple_delay,omitempty"`
			TokenBucket struct {
				Size           int   `json:"size,omitempty"`
				Limit          int   `json:"limit,omitempty"`
				ExpirationTime int64 `json:"expiration_time,omitempty"`
			} `json:"token_bucket,omitempty"`
		}
	}
	Loglevel    string `json:"loglevel,omitempty"`
	Log         string `json:"log,omitempty"`
	Csign       string `json:"csign,omitempty"`
	ForwardsMax int64  `json:"forwards_max,omitempty"`
	DataDir     string `json:"data_dir,omitempty"`
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

// Сообщение в irc
// TODO: Имплементировать тип сообщения - privmsg или action
type iMsg struct {
	ChatId string
	Text   string
}

// "Ведёрко" для реализации ограничителя скорости отправки исходящих сообщений - Token Bucket
type bucket struct {
	Timestamps []int64
	IsFull     bool
}

/* vim: set ft=go noet ai ts=4 sw=4 sts=4: */
