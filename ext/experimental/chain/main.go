package main

import (
	"fmt"
	"time"
)

func main() {
	// 使用缓冲通道异步处理消息
	msgCh := make(chan string, 100)
	go func() {
		defer close(msgCh)
		for {
			time.Sleep(1 * time.Second)
			msgCh <- "xxxxxxxxxxxxxxxxx:" + fmt.Sprint(time.Now().Unix())
		}
	}()

	for msg := range msgCh {
		fmt.Println(msg)
	}
}
