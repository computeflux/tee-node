package types

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
	in       chan T
	closed   bool
}

func NewPersistChan[T any](key string, cacheLen uint16) (*PersistChan[T], error) {
	pc := &PersistChan[T]{
		key:      key,
		list:     make([]T, 0),
		listChan: make(chan T, cacheLen),
		in:       make(chan T, cacheLen),
	}

	go pc.sender()
	err := pc.load()
	return pc, err
}

// sender 将内部队列中的消息按 FIFO 顺序转发到 listChan，
// 确保所有消息（包括加载的历史消息和新的 Push）顺序一致。
func (c *PersistChan[T]) sender() {
	for msg := range c.in {
		c.listChan <- msg
	}
	close(c.listChan)
}

func (c *PersistChan[T]) load() error {
	bt, err := SECRET_DB.GetKey("PersistChan", c.key)
	if err != nil && !errors.Is(err, pebble.ErrNotFound) {
		return fmt.Errorf("load persist chan: %w", err)
	}
	if errors.Is(err, pebble.ErrNotFound) {
		c.list = make([]T, 0)
		return nil
	}

	list := make([]T, 0)
	json.Unmarshal(bt, &list)
	c.mu.Lock()
	c.list = append(c.list, list...)
	c.mu.Unlock()

	for _, v := range list {
		c.in <- v
	}
	return nil
}

func (c *PersistChan[T]) save() error {
	bt, err := json.Marshal(c.list)
	if err != nil {
		return fmt.Errorf("save persist chan: %w", err)
	}
	return SECRET_DB.SetKey("PersistChan", c.key, bt)
}

func (c *PersistChan[T]) Push(msg T) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errors.New("persist channel is closed")
	}
	c.list = append(c.list, msg)
	if err := c.save(); err != nil {
		c.mu.Unlock()
		return err
	}
	c.in <- msg
	c.mu.Unlock()
	return nil
}

func (c *PersistChan[T]) Start(handler func(T) error) {
	for data := range c.listChan {
		c.mu.Lock()
		if len(c.list) > 0 {
			// 重新分配切片，避免内存泄漏（原 c.list[1:] 不会释放被弹出元素的内存）
			newList := make([]T, 0, len(c.list)-1)
			newList = append(newList, c.list[1:]...)
			c.list = newList
			c.save()
		}
		c.mu.Unlock()

		handler(data)
	}
}

func (c *PersistChan[T]) Stop() {
	c.mu.Lock()
	if !c.closed {
		c.closed = true
		close(c.in)
	}
	c.mu.Unlock()
}
