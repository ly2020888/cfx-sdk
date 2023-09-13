package main

import (
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"strconv"
	"time"

	sdk "github.com/Conflux-Chain/go-conflux-sdk"
	"github.com/Conflux-Chain/go-conflux-sdk/cfxclient/bulk"
	"github.com/Conflux-Chain/go-conflux-sdk/types"
	"github.com/Conflux-Chain/go-conflux-sdk/types/cfxaddress"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

const DEBUG = 1
const CFX1 = 1000000000000000 * 1000 / 1000000 //(1CFX / 1e6) 为单位
const CONCURRENCY int = 20
const BATCHSIZE int = 100

type Worker struct {
	address    string
	rate       int
	client     *sdk.Client
	sinal      *chan int
	froms      []cfxaddress.Address
	tos        []cfxaddress.Address
	bulkSender *bulk.BulkSender
}

func (worker *Worker) cfxCal(timeLimit uint) int {

	res := worker.random_transfer(timeLimit, BALANCE)

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

func (worker *Worker) updateAccount() {
	address := cfxaddress.MustNewFromHex("0x1e77b924efe10e49c7e9d9989adedfe41c8f2d38", 1234)
	err := am.Update(address, KEYDIR, "hello")
	if err != nil {
		fmt.Printf("update address error: %v \n\n", err)
		return
	}
	fmt.Printf("update address %v done\n\n", address)
}

func (worker *Worker) transfer(cfx1 types.Address, cfx2 types.Address, num int) {

	if len(worker.froms) < BATCHSIZE {
		worker.froms = append(worker.froms, cfx1)
		worker.tos = append(worker.froms, cfx2)
	} else {
		// unlock
		for _, user := range worker.froms {
			worker.client.AccountManager.Unlock(user, "hello")
		}

		tmp := big.NewInt(int64(num))
		value := tmp.Mul(tmp, big.NewInt(CFX1)) //1CFX
		res := (*hexutil.Big)(value)

		for i := 0; i < len(worker.froms); i++ {
			//创建未签名的交易
			utx, err := worker.client.CreateUnsignedTransaction(worker.froms[i], worker.tos[i], res, nil)
			if err != nil {
				panic(err)
			}
			nonce, _ := worker.client.TxPool().NextNonce(cfx1)
			utx.Nonce = nonce
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
	var total int = 0
	C := time.After(time.Duration(timeLimit) * time.Second)

	var trans = func(from int, to int) {
		worker.transfer(subLst[from], subLst[to], num)
		log.Default().Printf("from %v to %v\n", subLst[from], subLst[to])
		total++

	}

	for {
		select {
		case <-C:
			*worker.sinal <- int(total)
			log.Default().Printf("worker exited.")
			return int(total)
		default:
			from := rand.Int() % len(subLst)
			to := rand.Int() % len(subLst)
			trans(from, to)
		}

	}

}

// 账户的金额分配

func (worker *Worker) allocation(num int, money int) {
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
			tmp := worker.GetBalance(worker.address, adtmp)
			numcfx := tmp / (sz + 5)
			// 不均分保证有足够的gas
			for j := 0; j < sz; j++ {
				if worker.GetBalance(worker.address, adtmp) < numcfx+10 {
					break
				}
				fmt.Println("尝试进行交易: ", adtmp, " -> ", lst[j])
				worker.transfer(adtmp, lst[j], numcfx*1000000)
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

			for j := 0; j < sz; j++ {
				fmt.Println("尝试进行交易: ", adtmp, " -> ", lst[j])
				worker.transfer(adtmp, lst[j], money*1000000)
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
