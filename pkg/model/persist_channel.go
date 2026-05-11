package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/cockroachdb/pebble"
)

// 持久化消息队列示例，写入、读取文件
type PersistChan[T any] struct {
	key      string
	mu       sync.Mutex
	list     []T
	listChan chan T
}

func NewPersistChan[T any](key string, cacheLen uint16) (*PersistChan[T], error) {
	pc := &PersistChan[T]{
		key:      key,
		list:     make([]T, 0),
		listChan: make(chan T, cacheLen),
	}

	err := pc.load()
	return pc, err
}

func (c *PersistChan[T]) load() error {
	bt, err := GetKey("PersistChan", c.key)
	if err != nil && !errors.Is(err, pebble.ErrNotFound) {
		return fmt.Errorf("load persist chan: %w", err)
	}
	if errors.Is(err, pebble.ErrNotFound) {
		c.list = make([]T, 0)
		return nil
	}

	list := make([]T, 0)
	json.Unmarshal(bt, &list)
	for _, v := range list {
		c.Push(v)
	}

	return nil
}

func (c *PersistChan[T]) save() error {
	bt, err := json.Marshal(c.list)
	if err != nil {
		return fmt.Errorf("save persist chan: %w", err)
	}
	return SetKey("PersistChan", c.key, bt)
}

func (c *PersistChan[T]) Push(msg T) error {
	c.mu.Lock()
	c.list = append(c.list, msg)
	c.save()
	c.mu.Unlock()

	// write to chain
	c.listChan <- msg
	return nil
}

func (c *PersistChan[T]) Start(handler func(T) error) {
	for data := range c.listChan {
		c.mu.Lock()
		c.list = c.list[1:]
		c.save()
		c.mu.Unlock()

		handler(data)
		// if err != nil {
		// 	c.Push(data)
		// }
	}
}

func (c *PersistChan[T]) Stop() {
	close(c.listChan)
}
