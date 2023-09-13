package main

import (
	"log"
	"time"

	sdk "github.com/Conflux-Chain/go-conflux-sdk"
	"github.com/Conflux-Chain/go-conflux-sdk/cfxclient/bulk"
	"github.com/Conflux-Chain/go-conflux-sdk/types/cfxaddress"
)

var (
	am        *sdk.AccountManager
	TotalTime float64 = 0
)

const (
	KEYDIR  string = "./keystore"
	BALANCE int    = 21
	//21是燃油费 -> 0.000021CFX
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
			address:    config.Urls[i],
			rate:       config.Rate,
			client:     client,
			sinal:      &tb.sinal,
			bulkSender: bulk.NewBulkSender(*client),
			froms:      make([]cfxaddress.Address, 0),
			tos:        make([]cfxaddress.Address, 0),
		})
		//给账户分发钱
	}
	//挖矿节点有config.Numbers个，然后直接分发金额
	//一个账号初始化时转给100，那么每个账户得到的钱是：  节点数 * 100
	// tb.workers[0].allocation(config.Numbers, 100)
	/*
		cfx8 := cfxaddress.MustNew("0x1ebfeac86d8e997f768547a189fc30dc4b1b4dee")
		cfx10 := cfxaddress.MustNew("0x1f3f2e2abcc09653e3b03be9428f19f25472f38b")
		tb.workers[0].transfer(cfx8, cfx10, BALANCE)
	*/
	// tb.workers[0].GetBalance(config.Urls[0], cfxaddress.MustNewFromHex("0x1e77b924efe10e49c7e9d9989adedfe41c8f2d38", 1234))
	tb.start(config.Time)
} //

func (tb *TestBed) start(time1 uint) {
	startTime := time.Now()

	for i := 0; i < len(tb.workers); i++ {
		log.Default().Printf("worker %d started", i)
		go tb.workers[i].cfxCal(time1)
	}
	totalTransactions := 0
	for {
		totalTransactions += <-tb.sinal // 收到wokrer结束信号
		tb.counter++
		if tb.counter >= len(tb.workers) {
			log.Default().Printf("全部工人已经执行完工作\n")
			now := time.Now()
			t := now.Sub(startTime).Seconds()
			log.Default().Printf("执行时间%f, 总计交易数量%d,Tps:%f\n", t, totalTransactions, float64(totalTransactions)/t)
			return
		}

	}

}
