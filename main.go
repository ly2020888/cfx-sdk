package main

import (
	"log"

	sdk "github.com/Conflux-Chain/go-conflux-sdk"
)

var (
	am        *sdk.AccountManager
	TotalTime float64 = 0
)

const (
	KEYDIR  string = "./keystore"
	BALANCE int    = 1
)

type TestBed struct {
	conf    Config
	workers []Worker
	sinal   chan int
	counter int
}

func main() {
	config := NewConfig()
	tb := TestBed{
		conf:    config,
		workers: make([]Worker, 0, config.Numbers),
		sinal:   make(chan int, 1),
		counter: 0,
	}
	for i := 0; i < len(config.Urls); i++ {
		client, _ := sdk.NewClient(config.Urls[i])
		log.Default().Printf("%v", config.Urls[i])
		am = sdk.NewAccountManager(KEYDIR, 1234)
		client.SetAccountManager(am) //设置对应节点的账号管理器

		tb.workers = append(tb.workers, Worker{
			address: config.Urls[i],
			rate:    config.Rate,
			client:  client,
			sinal:   &tb.sinal,
		})
		//给账户分发钱
	}
	//挖矿节点有config.Numbers个，然后直接分发金额
	// tb.workers[0].allocation(config.Numbers)
	tb.start(config.Time)
} //

func (tb *TestBed) start(time uint) {
	for i := 0; i < len(tb.workers); i++ {
		go tb.workers[i].cfxCal(time)
	}

	for {
		<-tb.sinal // 收到wokrer结束信号
		tb.counter++
		if tb.counter >= len(tb.workers) {
			log.Default().Printf("全部工人已经执行完工作")
			return
		}

	}
}
