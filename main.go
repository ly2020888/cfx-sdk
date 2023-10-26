package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	sdk "github.com/Conflux-Chain/go-conflux-sdk"
	"github.com/Conflux-Chain/go-conflux-sdk/types/cfxaddress"
)

var (
	am        *PrivatekeyAccountManager
	TotalTime float64 = 0
)

const (
	KEYDIR string = "./keystore"
	//21是燃油费 -> 0.000021CFX
)

type TestBed struct {
	conf    Config
	workers []Worker
	sinal   chan int
	counter int
}

var file, err = os.OpenFile(".//log.txt", os.O_WRONLY|os.O_CREATE, 0644)

func main() {

	if err != nil {
		panic(err)
	}
	defer file.Close()

	config := NewConfig()
	tb := TestBed{
		conf:    config,
		workers: make([]Worker, 0, config.Numbers),
		sinal:   make(chan int, 1),
		counter: 0,
	}
	//am = NewPrivatekeyAccountManager(nil, 1234)
	//fmt.Println(am.Create("hello"))

	am = NewPrivatekeyAccountManager(nil, 1234) //创建账户管理器
	readAccountToAm()                           //将文件夹内的账户读入

	for i := 0; i < config.Numbers; i++ {
		client, _ := sdk.NewClient(config.Urls[i])
		log.Default().Printf("%v", config.Urls[i])
		//am = sdk.NewAccountManager(KEYDIR, 1234)
		//以私钥的形式导入
		//am = NewPrivatekeyAccountManager(nil, 1234)
		//fmt.Println(am.Import(KEYDIR, "hello", "hello"))
		//client.SetAccountManager(am) //设置对应节点的账号管理器
		//fmt.Println(len(am.List()))
		tb.workers = append(tb.workers, Worker{
			address: config.Urls[i],
			rate:    config.Rate,
			client:  client,
			sinal:   &tb.sinal,
			//			bulkSender: bulk.NewBulkSender(*client),
			froms: make([]cfxaddress.Address, 0),
			tos:   make([]cfxaddress.Address, 0),
		})
		tb.workers[i].client.SetAccountManager(am)
	}

	/*
		//测试代码 误删
		am = NewPrivatekeyAccountManager(nil, 1234)
		fmt.Println(am.Import("s\\UTC--2023-09-08T15-18-54.677331700Z--73bde97788d306ebe99962a24955c6fb59f342d0", "hello", "hello"))
		fmt.Println(am.Import("s\\UTC--2023-09-08T15-20-43.602074800Z--ae77b924efe10e49c7e9d9989adedfe41c8f2d38", "hello", "hello"))
		fmt.Println(len(am.List()))
		test := (*hexutil.Big)(big.NewInt(1000000000000000))
		tb.workers[0].tett(am.List()[0], am.List()[1], test)
	*/

	/*
		for i := 0; i < len(tb.workers); i++ {
			tb.workers[i].unlock()
		}
	*/
	//挖矿节点有config.Numbers个，然后直接分发金额
	//一个账号初始化时转给100，那么每个账户得到的钱是：  节点数 * 100
	//fmt.Println(len(am.List()))
	//tb.workers[0].allocation(config.Numbers, 100)

	tb.start(config.Time)
	time.Sleep(3 * time.Second)
	tb.workers[0].randomtransfer()
	time.Sleep(3 * time.Second)
	tb.workers[0].randomtransfer()
	time.Sleep(3 * time.Second)
	tb.workers[0].randomtransfer()
	time.Sleep(3 * time.Second)
	tb.workers[0].randomtransfer()
	time.Sleep(3 * time.Second)
	tb.workers[0].randomtransfer()

	//tb.workers[0].GetAllBalance()
}

func (tb *TestBed) start(timeLimit float64) {

	for i := 0; i < len(tb.workers); i++ {
		log.Default().Printf("worker %d started", i)
		go tb.workers[i].cfxCal(timeLimit, int(NewConfig().Peers))
	}
	//startTime := time.Now()

	totalTransactions := 0
	for {
		totalTransactions += <-tb.sinal // 收到wokrer结束信号
		tb.counter++
		if tb.counter >= len(tb.workers) {
			log.Default().Printf("全部工人已经执行完工作\n")
			/*	t := 0.0
				for i := 0; i < len(tb.workers); i++ {
					t += float64(tb.workers[i].pastTime)
				}
			*/
			t := timeLimit
			log.Default().Printf("执行时间%f, 总计交易数量%d,不合法交易数量:%d,Tps:%f,  上链情况:%d\n", t, totalTransactions, invalidTransactions, float64(totalTransactions)/t, onChain)
			return
		}

	}

}

func readAccountToAm() {
	dir := KEYDIR // 指定目录的路径

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	start := time.Now()
	for _, file := range files {
		filePath := filepath.Join(dir, file.Name())
		am.Import(filePath, "hello", "hello")
		//fmt.Println(am.Import(filePath, "hello", "hello"))
	}
	elapsed := time.Since(start)
	fmt.Printf("该段代码运行消耗的时间为：%s", elapsed)
	fmt.Println(len(am.List()))

}
