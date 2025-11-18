package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"strconv"
	"sync/atomic"
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
	BATCHSIZE   int   = 200
	BALANCE     int64 = 21
)

var singleTransfer *hexutil.Big = (*hexutil.Big)(big.NewInt(BALANCE * CFX1)) //1CFX
var invalidTransactions uint = 0
var totalCounter uint64 = 0

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

type BalanceInfo struct {
	Index   int    `json:"index"`
	Address string `json:"address"`
	Balance int64  `json:"balance"`
}

func (worker *Worker) unlock() {
	log.Default().Println("worker is unlocking the accounts")
	for _, user := range am.List() {
		worker.client.AccountManager.Unlock(user, "hello")
	}
}

func (worker *Worker) resetForRun() {
	worker.pastTime = 0
}
func (worker *Worker) cfxCal(ctx context.Context, startPeer int) int {
	//worker.client.SetAccountManager(am)
	res := worker.random_transfer(ctx, startPeer)
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

func (worker *Worker) GetBalance(url string, addres types.Address) int64 {
	balance, _ := worker.client.GetBalance(addres)
	dec, err := hexutil.DecodeBig(balance.String())
	//为了转换为CFX进行了除以1e3的运算，具体可以输出dec然后对照钱包余额自己推公式
	dec = dec.Div(dec, big.NewInt(1e3))
	if err != nil {
		panic(err)
	}
	tmp := dec.Int64()
	num := int64(tmp)
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

	nonce, err := worker.nextNonceFor(cfx1)
	if err != nil {
		fmt.Printf("get nonce failed: %v\n", err)
		return
	}
	begin := time.Now()
	utx, err := worker.client.CreateUnsignedTransaction(cfx1, cfx2, value, nil) //from, err := client.AccountManger()
	//nonce, err := worker.client.GetNextUsableNonce(cfx1)
	if err != nil {
		fmt.Printf("what utex is nil  %v", err)
	}
	overwriteTransactionNonce(&utx, nonce)
	//utx.Nonce = nonce

	_, err = worker.client.SendTransaction(utx)
	if err != nil {
		fmt.Println(err)
		invalidTransactions++
	}
	// _, err = worker.client.WaitForTransationReceipt(txhash, 5)
	// if err != nil {
	// 	fmt.Println(err)
	// 	//invalidTransactions++
	// }
	elapsed := time.Since(begin)
	worker.pastTime += elapsed.Seconds()
}

func (worker *Worker) nextNonceFor(addr types.Address) (*big.Int, error) {
	key := addr.String()
	bucket, _ := nonceCounters.LoadOrStore(key, &atomic.Int64{})
	counter := bucket.(*atomic.Int64)
	next := counter.Add(1) - 1
	if next < 0 {
		return nil, fmt.Errorf("nonce overflow for %s", key)
	}
	return big.NewInt(next), nil
}

func (worker *Worker) transfer2(cfx1 types.Address, cfx2 types.Address, value *hexutil.Big) {
	nonce, err := worker.nextNonceFor(cfx1)
	if err != nil {
		fmt.Printf("get nonce failed: %v\n", err)
		return
	}
	utx, err := worker.client.CreateUnsignedTransaction(cfx1, cfx2, value, nil) //from, err := client.AccountManger()
	overwriteTransactionNonce(&utx, nonce)
	if err != nil {
		fmt.Printf("%v", err)
	}
	_, err = worker.client.SendTransaction(utx)
	if err != nil {
		fmt.Println(err)
		invalidTransactions++
	}

}
func (worker *Worker) BatchTransfer(cfx1 types.Address, cfx2 types.Address, value *hexutil.Big) error {
	worker.froms = append(worker.froms, cfx1)
	worker.tos = append(worker.tos, cfx2)

	if len(worker.froms) < BATCHSIZE {
		return nil
	}

	if err := worker.flushBatchTransactions(value); err != nil {
		return err
	}

	return nil
}

func (worker *Worker) flushBatchTransactions(value *hexutil.Big) error {
	if len(worker.froms) == 0 {
		return nil
	}

	// ensure bulkSender initialized
	if worker.bulkSender == nil {
		if worker.client == nil {
			return fmt.Errorf("bulkSender nil and client is nil")
		}
		worker.bulkSender = bulk.NewBulkSender(*worker.client)
	}

	for i := 0; i < len(worker.froms); i++ {
		nonce, err := worker.nextNonceFor(worker.froms[i])
		if err != nil {
			return fmt.Errorf("get nonce failed: %w", err)
		}

		utx, err := worker.client.CreateUnsignedTransaction(worker.froms[i], worker.tos[i], value, nil)
		if err != nil {
			log.Default().Printf("create unsigned tx failed for %v -> %v: %v", worker.froms[i], worker.tos[i], err)
			continue
		}

		overwriteTransactionNonce(&utx, nonce)
		tx := utx // avoid taking address of loop variable
		worker.bulkSender.AppendTransaction(&tx)
	}

	hashes, errors, err := worker.bulkSender.SignAndSend()
	worker.bulkSender.Clear()
	if err != nil {
		return err
	}

	for i := 0; i < len(hashes); i++ {
		if errors[i] != nil {
			log.Default().Printf("sign and send the %vth tx error %v\n", i, errors[i])
		} else {
			log.Default().Printf("the %vth tx hash %v\n", i, hashes[i])
		}
	}

	worker.clearCache()
	return nil
}

func (worker *Worker) flushPendingTransactions(value *hexutil.Big) {
	if len(worker.froms) == 0 {
		return
	}

	if err := worker.flushBatchTransactions(value); err != nil {
		log.Default().Printf("flush pending transactions error: %v", err)
		worker.clearCache()
	}
}

func (worker *Worker) random_transfer(ctx context.Context, startPeer int) int {
	//打乱账户顺序，交易金额为num

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	lst := am.List()
	//从几号节点开始
	subLst := make([]types.Address, len(lst)-startPeer)
	copy(subLst, lst[startPeer:])
	//对lst实现随机洗牌-打乱顺序
	rng.Shuffle(len(subLst), func(i, j int) {
		subLst[i], subLst[j] = subLst[j], subLst[i]
	})
	var total int = 0
	workerRunStart := time.Now()
	var trans = func(from int, to int) {
		worker.BatchTransfer(subLst[from], subLst[to], singleTransfer)
		log.Default().Printf("from %v to %v\n", subLst[from], subLst[to])
		total++
		atomic.AddUint64(&totalCounter, 1)
		elapsed := time.Since(workerRunStart).Seconds()
		_, err = file.WriteString(fmt.Sprintf("ctx过去%f秒, pastTime过去%f秒  多节点总共完成%d笔交易\n", elapsed, worker.pastTime, totalCounter))

	}

	for {
		select {
		case <-ctx.Done():
			worker.flushPendingTransactions(singleTransfer)
			*worker.sinal <- int(total)
			log.Default().Printf("worker exited: %v", ctx.Err())
			return int(total)
		default:
		}

		if len(subLst) == 0 {
			log.Default().Printf("worker exited: no available accounts")
			*worker.sinal <- int(total)
			return int(total)
		}

		from := rng.Intn(len(subLst))
		to := rng.Intn(len(subLst))
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
			numcfx := tmp / int64(sz+5)
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

func cloneHexutilBig(value *hexutil.Big) *hexutil.Big {
	if value == nil {
		return nil
	}
	copyInt := new(big.Int).Set(value.ToInt())
	cloned := hexutil.Big(*copyInt)
	return &cloned
}

func (worker *Worker) GetAllBalance() []BalanceInfo {
	lst := am.List()
	balances := make([]BalanceInfo, 0, len(lst))
	for i := 0; i < len(lst); i++ {
		num := worker.GetBalance(worker.address, lst[i])
		log.Printf("账户%d, 目前有%v钱数\n", i, num)
		balances = append(balances, BalanceInfo{
			Index:   i,
			Address: lst[i].String(),
			Balance: num,
		})
	}
	return balances
}

func overwriteTransactionNonce(tx *types.UnsignedTransaction, nonce *big.Int) {
	if tx == nil || nonce == nil {
		return
	}
	tmp := hexutil.Big(*big.NewInt(0).Set(nonce))
	tx.Nonce = &tmp
}
