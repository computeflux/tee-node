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
	key       string
	db        *DB // 构造时捕获的数据库实例，避免读取可变的全局 SECRET_DB
	mu        sync.Mutex
	list      []T
	listChan  chan T
	in        chan T // 永不关闭；停止时通过 stopped 信号让 sender/Start 退出
	stopped   chan struct{}
	closed    bool
	started   bool
	startDone chan struct{} // Start 处理循环退出时关闭，Stop 等待它保证消息全部处理完
}

func NewPersistChan[T any](key string, cacheLen uint16) (*PersistChan[T], error) {
	pc := &PersistChan[T]{
		key:       key,
		db:        SECRET_DB, // 捕获构造时的全局 DB；后续 SECRET_DB 被重新赋值不影响本队列
		list:      make([]T, 0),
		listChan:  make(chan T, cacheLen),
		in:        make(chan T, cacheLen),
		stopped:   make(chan struct{}),
		startDone: make(chan struct{}),
	}

	go pc.sender()
	err := pc.load()
	return pc, err
}

// sender 将内部队列中的消息按 FIFO 顺序转发到 listChan，
// 确保所有消息（包括加载的历史消息和新的 Push）顺序一致。
// 收到停止信号后把 in 中已缓冲的消息转发完再退出并关闭 listChan。
func (c *PersistChan[T]) sender() {
	defer close(c.listChan)
	for {
		select {
		case msg := <-c.in:
			c.listChan <- msg
		case <-c.stopped:
			for {
				select {
				case msg := <-c.in:
					c.listChan <- msg
				default:
					return
				}
			}
		}
	}
}

func (c *PersistChan[T]) load() error {
	bt, err := c.db.GetKey("PersistChan", c.key)
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
	return c.db.SetKey("PersistChan", c.key, bt)
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
	c.mu.Unlock()

	// 锁外投递，避免 in 满时持锁阻塞导致 Start 循环无法弹出消息而死锁。
	// 若投递时通道已停止，消息直接丢弃（消息已持久化，重启后 load 会恢复）。
	select {
	case c.in <- msg:
	case <-c.stopped:
	}
	return nil
}

// popLocked 弹出并持久化 list 头部元素，返回弹出的消息。
// 必须在持有 c.mu 时调用。
func (c *PersistChan[T]) popLocked() T {
	var zero T
	if len(c.list) == 0 {
		return zero
	}
	msg := c.list[0]
	// 重新分配切片，避免内存泄漏（原 c.list[1:] 不会释放被弹出元素的内存）
	newList := make([]T, 0, len(c.list)-1)
	newList = append(newList, c.list[1:]...)
	c.list = newList
	c.save()
	return msg
}

func (c *PersistChan[T]) Start(handler func(T) error) {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return
	}
	c.started = true
	closed := c.closed
	c.mu.Unlock()

	defer close(c.startDone)

	if closed {
		// Stop 已先于本 Start 返回：调用方可能已关闭 DB，禁止任何 DB 写入。
		// 缓冲中的消息仍持久化在 DB 里，下次启动 load 时会重新入队处理，
		// 因此直接丢弃内存缓冲即可。
		// 必须排空 listChan，否则 sender 可能因缓冲区满而阻塞泄漏。
		for range c.listChan {
		}
		return
	}

	for {
		select {
		case data, ok := <-c.listChan:
			if !ok {
				return
			}
			c.mu.Lock()
			c.popLocked()
			c.mu.Unlock()

			handler(data)
		case <-c.stopped:
			// 停止信号：把 listChan 中剩余消息全部处理完再退出
			for data := range c.listChan {
				c.mu.Lock()
				c.popLocked()
				c.mu.Unlock()

				handler(data)
			}
			return
		}
	}
}

func (c *PersistChan[T]) Stop() {
	c.mu.Lock()
	if !c.closed {
		c.closed = true
		close(c.stopped)
	}
	started := c.started
	c.mu.Unlock()

	// 等待 Start 处理循环把缓冲消息全部处理完，
	// 避免调用方在 Stop 返回后立即关闭底层 DB 时仍有后台 goroutine 在写入。
	if started {
		<-c.startDone
	}
}
