package main

import (
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"strconv"
	"time"

	sdk "github.com/Conflux-Chain/go-conflux-sdk"
	"github.com/Conflux-Chain/go-conflux-sdk/types"
	"github.com/Conflux-Chain/go-conflux-sdk/types/cfxaddress"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

const DEBUG = 1
const CFX1 = 1000000000000000 * 1000 / 1000000 //(1CFX / 1e6) 为单位

type Worker struct {
	address string
	rate    int
	client  *sdk.Client
	sinal   *chan int
}

func (woker *Worker) cfxCal(timeLimit uint) int {

	res := woker.random_transfer(timeLimit, BALANCE)

	fmt.Println("id: " + woker.address + "     交易次数:  " + strconv.Itoa(res))
	return res
}

func (woker *Worker) addToDir(privateKey string) {
	//	fmt.Println(len(am.List())) 获取账号个数
	cfx1, err := woker.client.AccountManager.ImportKey(privateKey, "hello")
	log.Default().Println(cfx1)
	if err != nil {
		panic(err)
	}
}

func (woker *Worker) GetBalance(url string, addres types.Address) int {
	balance, _ := woker.client.GetBalance(addres)
	dec, err := hexutil.DecodeBig(balance.String())
	//为了转换为CFX进行了除以1e3的运算，具体可以输出dec然后对照钱包余额自己推公式
	dec = dec.Div(dec, big.NewInt(1e3))
	if DEBUG == 1 {
		//	fmt.Println("drip 单位 : ", addres, "  : ", dec)
	}
	if err != nil {
		panic(err)
	}
	tmp := dec.Int64()
	num := int(tmp)
	//fmt.Println(num)
	return num

}

func (woker *Worker) updateAccount() {
	address := cfxaddress.MustNewFromHex("0x1e77b924efe10e49c7e9d9989adedfe41c8f2d38", 1234)
	err := am.Update(address, KEYDIR, "hello")
	if err != nil {
		fmt.Printf("update address error: %v \n\n", err)
		return
	}
	fmt.Printf("update address %v done\n\n", address)
}

func (woker *Worker) transfer(cfx1 types.Address, cfx2 types.Address, num int) {
	//A账户到B账户
	tmp := big.NewInt(int64(num))
	value := tmp.Mul(tmp, big.NewInt(CFX1)) //1CFX
	res := (*hexutil.Big)(value)
	//解锁两个账户
	//	fmt.Println("查看大小： ", len(am.List()))
	woker.client.SetAccountManager(am)
	am.Unlock(cfx1, "hello")
	am.Unlock(cfx2, "hello")
	start := time.Now()
	//创建未签名的交易
	utx, err := woker.client.CreateUnsignedTransaction(cfx1, cfx2, res, nil) //from, err := client.AccountManger()

	if err != nil {
		panic(err)
	}
	//txhash, err := client.SendTransaction(utx)//输出交易的哈希值

	//对未签名的交易进行签名
	txhash, err := woker.client.SendTransaction(utx)
	//	fmt.Println(cfx1, " -> ", cfx2, " : 交易哈希为 ： ", txhash)
	elapsed := time.Since(start)
	duration := time.Duration(elapsed) * time.Nanosecond // 将纳秒转换为 time.Duration 类型
	TotalTime += duration.Seconds()                      //以秒为单位
	//	fmt.Printf("send transaction hash: %v\n\n", txhash)
	if err != nil {
		panic(err)
	}
	if txhash == "" {
		fmt.Println("交易失败")
	}
}

func (worker *Worker) random_transfer(timeLimit uint, num int) int {
	//打乱账户顺序，交易金额为num

	lst := am.List()
	//从几号节点开始
	startPerr := 4
	subLst := make([]types.Address, len(lst)-startPerr)
	copy(subLst, lst[startPerr:])
	//对lst实现随机洗牌-打乱顺序
	rand.Seed(time.Now().UnixNano())
	// 注意，这行重要，为了使每次洗牌的结果不一样，需要用不同的随机种子，我们这里用精确到微秒的时间戳
	rand.Shuffle(len(subLst), func(i, j int) {
		subLst[i], subLst[j] = subLst[j], subLst[i]
	})
	total := 0
	InOneSecond := 0
	C := time.After(time.Duration(timeLimit) * time.Second)

	for {
		startTime := time.Now()
		select {
		case <-C:
			*worker.sinal <- 1
			log.Default().Printf("worker exited.")
			return total
		default:
			from := rand.Int() % len(subLst)
			to := rand.Int() % len(subLst)
			worker.transfer(subLst[from], subLst[to], num)
			//log.Default().Printf("转账前  %v   %v \n", worker.GetBalance(worker.address, subLst[from]), worker.GetBalance(worker.address, subLst[to]))
			log.Default().Printf("from %v to %v\n", subLst[from], subLst[to])
			//	log.Default().Printf("转账后  %v   %v", worker.GetBalance(worker.address, subLst[from]), worker.GetBalance(worker.address, subLst[to]))

			total++

			InOneSecond++
			if InOneSecond >= worker.rate && worker.rate != 0 { // rate == 0, no limits
				now := time.Now()
				limitTime := startTime.Add(1 * time.Second)
				if limitTime.Sub(now) > 0 {
					time.Sleep(limitTime.Sub(now))
					InOneSecond = 0
				}
			}
		}

	}

}

// 账户的金额分配

func (woker *Worker) allocation(num int, money int) {
	//几个节点-num就是几
	//num : 2 4 8 16
	if money == -1 {
		all := am.List()
		lst := make([]types.Address, len(all)-num)
		copy(lst, all[num:]) //要分发钱的账户
		account := make([]types.Address, num)
		copy(account, am.List()[:num])
		sz := len(lst)

		var sinal chan int = make(chan int, 1)

		var allo = func(i int) {
			adtmp := account[i]
			tmp := woker.GetBalance(woker.address, adtmp)
			numcfx := tmp / (sz + 5)
			// 不均分保证有足够的gas
			for j := 0; j < sz; j++ {
				if woker.GetBalance(woker.address, adtmp) < numcfx+10 {
					break
				}
				fmt.Println("尝试进行交易: ", adtmp, " -> ", lst[j])
				woker.transfer(adtmp, lst[j], numcfx*1000000)
			}
			sinal <- 1

		}

		for i := 0; i < num; i++ {
			go allo(i)
		}
		counter := 0
		for {
			counter += <-sinal
			log.Default().Printf("allocation thread %d 's work has done", counter)
			if counter > num-1 {
				break
			}
		}
	} else {
		all := am.List()
		lst := make([]types.Address, len(all)-num)
		copy(lst, all[num:]) //要分发钱的账户
		account := make([]types.Address, num)
		copy(account, am.List()[:num])
		sz := len(lst)

		var sinal chan int = make(chan int, 1)

		var allo = func(i int) {
			adtmp := account[i]
			//自己给出money值，保证程序能够正确执行且各个账户余额足够交易
			woker.GetBalance(woker.address, adtmp)

			for j := 0; j < sz; j++ {
				fmt.Println("尝试进行交易: ", adtmp, " -> ", lst[j])
				woker.transfer(adtmp, lst[j], money*1000000)
			}
			sinal <- 1

		}

		for i := 0; i < num; i++ {
			go allo(i)
		}
		counter := 0
		for {
			counter += <-sinal
			log.Default().Printf("allocation thread %d 's work has done", counter)
			if counter > num-1 {
				break
			}
		}
	}

}
