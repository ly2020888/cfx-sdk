package main

import (
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"strconv"
	"sync"
	"time"

	sdk "github.com/Conflux-Chain/go-conflux-sdk"
	"github.com/Conflux-Chain/go-conflux-sdk/cfxclient/bulk"
	"github.com/Conflux-Chain/go-conflux-sdk/types"
	"github.com/Conflux-Chain/go-conflux-sdk/types/cfxaddress"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

const (
	DEBUG       int   = 1
	CFX1        int64 = 1e18 / 1e6 //(1CFX / 1e6) 为单位
	CONCURRENCY int   = 20
	BATCHSIZE   int   = 100
	BALANCE     int64 = 21
)

var singleTransfer *hexutil.Big = (*hexutil.Big)(big.NewInt(BALANCE * CFX1)) //1CFX
var invalidTransactions uint = 0
var onChain uint = 0
var totalCounter uint = 0
var mutexTotalCounter sync.Mutex
var autoNonce *big.Int
var mutexAtuoNoncer sync.Mutex

type Worker struct {
	address    string
	rate       int
	client     *sdk.Client
	sinal      *chan int
	froms      []cfxaddress.Address
	tos        []cfxaddress.Address
	bulkSender *bulk.BulkSender
	pastTime   float64
}

func (worker *Worker) unlock() {
	log.Default().Println("worker is unlocking the accounts")
	for _, user := range am.List() {
		worker.client.AccountManager.Unlock(user, "hello")
	}
}
func (worker *Worker) cfxCal(timeLimit float64, startPeer int) int {
	//worker.client.SetAccountManager(am)
	autoNonce = big.NewInt(0)
	res := worker.random_transfer(timeLimit, startPeer)
	fmt.Println("id: " + worker.address + "     交易次数:  " + strconv.Itoa(res))
	return res
}

func (worker *Worker) addToDir(privateKey string) {
	//	fmt.Println(len(am.List())) 获取账号个数
	cfx1, err := worker.client.AccountManager.ImportKey(privateKey, "hello")
	log.Default().Println(cfx1)
	if err != nil {
		panic(err)
	}
}

func (worker *Worker) GetBalance(url string, addres types.Address) int {
	balance, _ := worker.client.GetBalance(addres)
	dec, err := hexutil.DecodeBig(balance.String())
	//为了转换为CFX进行了除以1e3的运算，具体可以输出dec然后对照钱包余额自己推公式
	dec = dec.Div(dec, big.NewInt(1e3))
	if err != nil {
		panic(err)
	}
	tmp := dec.Int64()
	num := int(tmp)
	//fmt.Println(num)
	return num

}

func (worker *Worker) updateAccount() {
	address := cfxaddress.MustNewFromHex("0x1e77b924efe10e49c7e9d9989adedfe41c8f2d38", 1234)
	err := am.Update(address, KEYDIR, "hello")
	if err != nil {
		fmt.Printf("update address error: %v \n\n", err)
		return
	}
	fmt.Printf("update address %v done\n\n", address)
}

func (worker *Worker) transfer(cfx1 types.Address, cfx2 types.Address, value *hexutil.Big) {
	//开始计时

	//mutexAtuoNoncer.Lock()
	//autoNonce.Add(autoNonce, big.NewInt(1))
	//tmp := autoNonce
	//	atuoNonce++
	//mutexAtuoNoncer.Unlock()

	begin := time.Now()
	utx, err := worker.client.CreateUnsignedTransaction(cfx1, cfx2, value, nil) //from, err := client.AccountManger()
	nonce, err := worker.client.GetNextUsableNonce(cfx1)
	//	utx.Nonce.ToInt().Set(tmp)
	utx.Nonce = nonce
	if err != nil {
		fmt.Printf("%v", err)
	}
	_, err = worker.client.SendTransaction(utx)
	if err != nil {
		fmt.Println(err)
		invalidTransactions++
	}

	elapsed := time.Since(begin)
	worker.pastTime += elapsed.Seconds()

	/*
	   receipt, err := worker.client.GetTransactionReceipt(txhash)
	   	if err != nil {
	   		fmt.Println(err)
	   	}
	   	if receipt != nil {
	   		onChain++
	   	}
	*/
}
func (worker *Worker) transfer2(cfx1 types.Address, cfx2 types.Address, value *hexutil.Big) {
	//开始计时

	utx, err := worker.client.CreateUnsignedTransaction(cfx1, cfx2, value, nil) //from, err := client.AccountManger()
	//nonce, err := worker.client.GetNextUsableNonce(cfx1)
	if err != nil {
		fmt.Printf("%v", err)
	}
	_, err = worker.client.SendTransaction(utx)
	if err != nil {
		fmt.Println(err)
		invalidTransactions++
	}

}
func (worker *Worker) BatchTransfer(cfx1 types.Address, cfx2 types.Address, value *hexutil.Big) {

	if len(worker.froms) < BATCHSIZE {
		worker.froms = append(worker.froms, cfx1)
		worker.tos = append(worker.froms, cfx2)
	} else {

		for i := 0; i < len(worker.froms); i++ {
			//创建未签名的交易
			utx, err := worker.client.CreateUnsignedTransaction(worker.froms[i], worker.tos[i], value, nil)
			if err != nil {
				continue
			}
			worker.bulkSender.AppendTransaction(&utx)
		}

		worker.clearCache()
		hashes, errors, err := worker.bulkSender.SignAndSend()
		worker.froms = make([]cfxaddress.Address, 0)
		if err != nil {
			panic(fmt.Sprintf("%+v", err))
		}
		for i := 0; i < len(hashes); i++ {
			if errors[i] != nil {
				log.Default().Printf("sign and send the %vth tx error %v\n", i, errors[i])
			} else {
				log.Default().Printf("the %vth tx hash %v\n", i, hashes[i])
			}
		}
	}

}

func (worker *Worker) random_transfer(timeLimit float64, startPeer int) int {
	//打乱账户顺序，交易金额为num

	lst := am.List()
	//从几号节点开始
	subLst := make([]types.Address, len(lst)-startPeer)
	copy(subLst, lst[startPeer:])
	//对lst实现随机洗牌-打乱顺序
	rand.Seed(time.Now().UnixNano())
	// 注意，这行重要，为了使每次洗牌的结果不一样，需要用不同的随机种子，我们这里用精确到微秒的时间戳
	rand.Shuffle(len(subLst), func(i, j int) {
		subLst[i], subLst[j] = subLst[j], subLst[i]
	})
	var total int = 0
	//	C := time.After(time.Duration(timeLimit) * time.Second)
	//begin := time.Now()
	var trans = func(from int, to int) {
		worker.transfer(subLst[from], subLst[to], singleTransfer)
		log.Default().Printf("from %v to %v\n", subLst[from], subLst[to])
		total++
		mutexTotalCounter.Lock()
		totalCounter++
		mutexTotalCounter.Unlock()
		//	elapsed := time.Since(begin)
		log.Default().Printf("过去%v,  多节点总共完成%d笔交易\n", worker.pastTime, totalCounter)

	}

	for {
		if worker.pastTime > timeLimit {
			*worker.sinal <- int(total)
			log.Default().Printf("worker exited.")
			return int(total)
		}

		from := rand.Int() % len(subLst)
		to := rand.Int() % len(subLst)
		trans(from, to)

	}

}

// 账户的金额分配

func (worker *Worker) allocation(num int, money int) {
	//几个节点-num就是几
	//num : 2 4 8 16
	//	worker.client.SetAccountManager(am)
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
			tmp := worker.GetBalance(worker.address, adtmp)
			numcfx := tmp / (sz + 5)
			fees := big.NewInt(int64(numcfx * 1000000))
			fees.Mul(fees, big.NewInt(CFX1))

			for j := 0; j < sz; j++ {
				if worker.GetBalance(worker.address, adtmp) < numcfx+10 {
					break
				}
				fmt.Println("尝试进行交易: ", adtmp, " -> ", lst[j])

				worker.transfer(adtmp, lst[j], (*hexutil.Big)(fees))
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
			worker.GetBalance(worker.address, adtmp)
			fees := big.NewInt(int64(money * 1000000))
			fees.Mul(fees, big.NewInt(CFX1))
			for j := 0; j < sz; j++ {
				fmt.Println("尝试进行交易: ", adtmp, " -> ", lst[j])
				worker.transfer2(adtmp, lst[j], (*hexutil.Big)(fees))
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
func (worker *Worker) clearCache() {
	worker.froms = make([]cfxaddress.Address, 0)
	worker.tos = make([]cfxaddress.Address, 0)
}
