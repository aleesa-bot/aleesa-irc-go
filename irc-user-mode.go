package main

import "strings"

/* По сути, сбор mode-ов пользователей нам пока нужен только для того, чтобы определять следующие вещи:
- Может ли nick для определённого канала воспользоваться командой admin
- Применять ли ограничения на исходящие сообщения (пока что в состоянии todo.) MODE-ы +v и +o для бота на libera.chat
  активируют мягкий ratelimit на стороне сервера, без них, сервер просто рвёт соединение по превышению лимитов
*/

// Удаляет сведения о MODE-ах пользователя для заданного канала
func userModeDeleteUser(ircChan string, nick string) {
	channel, ok := userMode.Get(ircChan)

	if ok {
		channelData := channel.(map[string]map[string]bool)
		delete(channelData, nick)

		if len(channelData) == 0 {
			userMode.Delete(ircChan)
		} else {
			userMode.Set(ircChan, channelData)
		}
	}
}

// Удаляет сведения о MODE-ах пользователя для всех каналов, на которых есть бот
func userModePurgeUser(nick string) {
	for _, channel := range config.Irc.Channels {
		userModeDeleteUser(channel, nick)
	}
}

// Обновляет или создаёт запись о mode-ах пользователей
func userModeUpdateUser(ircChan string, nick string, modes string) {
	var channelData map[string]map[string]bool
	channel, ok := userMode.Get(ircChan)

	if ok {
		channelData = channel.(map[string]map[string]bool)
	}

	if channelData == nil {
		channelData = make(map[string]map[string]bool)
	}

	if channelData[nick] == nil {
		channelData[nick] = make(map[string]bool)
	}

	switch {
	case modes[:1] == "+":
		for _, mode := range strings.Split(modes[1:], "") {
			channelData[nick][mode] = true
		}
	case modes[:1] == "-":
		for _, mode := range strings.Split(modes[1:], "") {
			channelData[nick][mode] = false
		}
	}

	userMode.Set(ircChan, channelData)
}

// Возвращает значение MODE-а v для запрошенного ника
func userModeIsVoiced(ircChan string, nick string) bool {
	var channelData map[string]map[string]bool
	channel, ok := userMode.Get(ircChan)

	if ok {
		channelData = channel.(map[string]map[string]bool)
	} else {
		return false
	}

	if channelData == nil {
		return false
	}

	if channelData[nick] == nil {
		return false
	}

	if channelData[nick]["v"] {
		return true
	}

	return false
}

// Возвращает значение MODE-а o для запрошенного ника
func userModeIsOped(ircChan string, nick string) bool {
	var channelData map[string]map[string]bool
	channel, ok := userMode.Get(ircChan)

	if ok {
		channelData = channel.(map[string]map[string]bool)
	} else {
		return false
	}

	if channelData == nil {
		return false
	}

	if channelData[nick] == nil {
		return false
	}

	if channelData[nick]["o"] {
		return true
	}

	return false
}

func userModeIsHere(ircChan string, nick string) bool {
	var channelData map[string]map[string]bool
	channel, ok := userMode.Get(ircChan)

	if ok {
		channelData = channel.(map[string]map[string]bool)
	} else {
		return false
	}

	if channelData == nil {
		return false
	}

	if channelData[nick] != nil {
		return true
	}

	return false
}

/* vim: set ft=go noet ai ts=4 sw=4 sts=4: */
