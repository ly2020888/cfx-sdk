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

type Worker struct {
	address string
	rate    int
	client  *sdk.Client
	sinal   *chan int
}

func (woker *Worker) cfxCal(timeLimit uint) int {

	res := woker.random_transfer(timeLimit, BALANCE)

	fmt.Println("id: " + woker.address + "交易次数: " + strconv.Itoa(res))
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
	dec = dec.Div(dec, big.NewInt(1000000000000000*1000))
	if err != nil {
		panic(err)
	}
	tmp := dec.Int64()
	num := int(tmp)
	fmt.Println(num)
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
	value := tmp.Mul(tmp, big.NewInt(1000000000000000*1000)) //1CFX
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
	fmt.Println(cfx1, " -> ", cfx2, " : 交易哈希为 ： ", txhash)
	elapsed := time.Since(start)
	duration := time.Duration(elapsed) * time.Nanosecond // 将纳秒转换为 time.Duration 类型
	TotalTime += duration.Seconds()                      //以秒为单位
	//	fmt.Printf("send transaction hash: %v\n\n", txhash)
	if err != nil {
		panic(err)
	}
}

func (worker *Worker) random_transfer(timeLimit uint, num int) int {
	//打乱账户顺序，交易金额为num

	lst := am.List()
	fmt.Println(len(lst))
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
			return total
		default:
			from := rand.Int() % len(subLst)
			to := rand.Int() % len(subLst)
			worker.transfer(subLst[from], subLst[to], num)
			log.Default().Printf("from %v to %v", subLst[from], subLst[to])
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

func (woker *Worker) allocation(num int) {
	//几个节点-num就是几
	//num : 2 4 8 16
	all := am.List()

	lst := make([]types.Address, len(all)-num)
	copy(lst, all[num:]) //要分发钱的账户
	account := make([]types.Address, num)
	copy(account, am.List()[:num])
	sz := len(lst)
	fmt.Println("debug : ", sz)
	for i := 0; i < num; i++ {
		fmt.Println(account[i].GetHexAddress())
		adtmp := account[i]
		tmp := woker.GetBalance(woker.address, adtmp)
		numcfx := tmp / (sz + 5)
		// 不均分保证有足够的gas
		for j := 0; j < sz; j++ {
			if woker.GetBalance(woker.address, adtmp) < numcfx+10 {
				break
			}
			fmt.Println("目前账户余额: ", woker.GetBalance(woker.address, adtmp))
			woker.transfer(adtmp, lst[j], numcfx)
			fmt.Println("实现转账 : ", adtmp, lst[j])
		}
	}
}
