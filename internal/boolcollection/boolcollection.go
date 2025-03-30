package boolcollection

import (
	"sync"
)

// Collection это sync.Map-ка, хранящая в себе итемы типа bool.
type Collection struct {
	items sync.Map
	close chan struct{}
}

// Item представляет собой интерфейс для хранения данных типа bool.
type item struct {
	data bool
}

// NewCollection создаёт инстанс коллекции.
func NewCollection() *Collection {
	c := &Collection{
		close: make(chan struct{}),
	}

	return c
}

// Get достаёт данные по заданному ключу из коллекции.
func (collection *Collection) Get(key string) (bool, bool) {
	obj, exists := collection.items.Load(key)

	if !exists {
		return false, false
	}

	item := obj.(item)

	return item.data, true
}

// Set сохраняет данные с заданным ключом в коллекцию.
func (collection *Collection) Set(key string, value bool) {
	collection.items.Store(key, item{
		data: value,
	})
}

// Delete удаляет ключ и значение из коллекции данных.
func (collection *Collection) Delete(key string) {
	collection.items.Delete(key)
}

// Close очищает и высвобождает ресурсы, занятые коллекцией.
func (collection *Collection) Close() {
	collection.close <- struct{}{}
	collection.items = sync.Map{}
}

/* vim: set ft=go noet ai ts=4 sw=4 sts=4: */
