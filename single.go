package main

import (
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/Conflux-Chain/go-conflux-sdk/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var single_Transfer *hexutil.Big = (*hexutil.Big)(big.NewInt(BALANCE * CFX1 * 1000)) //1CFX
var file2, err2 = os.OpenFile(".//single.txt", os.O_WRONLY|os.O_CREATE, 0644)
var nonceCounters sync.Map // key: account hex string, value: *atomic.Int64 maintained in worker.go

func (worker *Worker) wait_single_transfer(cfx1 types.Address, cfx2 types.Address, value *hexutil.Big) {
	//开始计时
	// fmt.Println("开始单账户转账测试")
	// intValue := int(value.ToInt().Int64())
	// fmt.Println("what now:", cfx1, cfx2, "transfer value:", intValue)
	nonce, err := worker.nextNonceFor(cfx1)
	if err != nil {
		fmt.Printf("get nonce failed: %v\n", err)
		return
	}
	begin := time.Now()
	utx, err := worker.client.CreateUnsignedTransaction(cfx1, cfx2, value, nil) //from, err := client.AccountManger()
	//nonce, err := worker.client.GetNextUsableNonce(cfx1)
	if err != nil {
		fmt.Printf("what utx %v", err)
	}
	// utx.Nonce.ToInt().Set(nonce)
	//	utx.Nonce = nonce
	overwriteTransactionNonce(&utx, nonce)

	txhash, err := worker.client.SendTransaction(utx)
	if err != nil {
		fmt.Println(err)
		//invalidTransactions++
	}
	receipt, err := worker.client.WaitForTransationReceipt(txhash, 5)
	if err != nil {
		fmt.Println(err)
		//invalidTransactions++
	}
	elapsed := time.Since(begin)
	_, err = file2.WriteString(fmt.Sprintf("交易情况%v\n", receipt))
	if err != nil {
		fmt.Println(err)
		//invalidTransactions++
	}
	_, err = file2.WriteString(fmt.Sprintf("过去%v,  完成1笔交易\n", elapsed))
	if err != nil {
		fmt.Println("file error", err)
		//invalidTransactions++
		return
	}
	// fmt.Println("单账户转账测试完成")
}

func (worker *Worker) single_transfer(cfx1 types.Address, cfx2 types.Address, value *hexutil.Big) {
	//开始计时
	// fmt.Println("开始单账户转账测试")
	intValue := int(value.ToInt().Int64())
	fmt.Println("what now:", cfx1, cfx2, "transfer value:", intValue)
	nonce, err := worker.nextNonceFor(cfx1)
	if err != nil {
		fmt.Printf("get nonce failed: %v\n", err)
		return
	}

	begin := time.Now()
	utx, err := worker.client.CreateUnsignedTransaction(cfx1, cfx2, value, nil) //from, err := client.AccountManger()
	//nonce, err := worker.client.GetNextUsableNonce(cfx1)
	if err != nil {
		fmt.Printf("what utx %v", err)
	}
	// utx.Nonce.ToInt().Set(nonce)
	//utx.Nonce = nonce
	overwriteTransactionNonce(&utx, nonce)

	_, err = worker.client.SendTransaction(utx)
	if err != nil {
		fmt.Println(err)
		invalidTransactions++
	}

	elapsed := time.Since(begin)
	worker.pastTime += elapsed.Seconds()

}

func (worker *Worker) randomtransfer() {
	//打乱账户顺序，交易金额为num
	lst := worker.accountPool()
	if len(lst) == 0 {
		log.Default().Printf("single transfer skipped: no accounts assigned")
		return
	}
	//从几号节点开始

	var trans = func(from int, to int) {
		worker.single_transfer(lst[from], lst[to], single_Transfer)
		//		log.Default().Printf("from %v to %v\n", subLst[from], subLst[to])
		//	elapsed := time.Since(begin)
		//	log.Default().Printf("过去%v,  多节点总共完成%d笔交易\n", worker.pastTime, totalCounter)
	}
	a := rand.Int() % len(lst)
	b := rand.Int() % len(lst)
	trans(a, b)
}

func (worker *Worker) LatencyRandomTransfer() {
	//打乱账户顺序，交易金额为num
	lst := worker.accountPool()
	if len(lst) == 0 {
		log.Default().Printf("latency transfer skipped: no accounts assigned")
		return
	}
	//从几号节点开始

	var trans = func(from int, to int) {
		worker.wait_single_transfer(lst[from], lst[to], single_Transfer)
		//		log.Default().Printf("from %v to %v\n", subLst[from], subLst[to])
		//	elapsed := time.Since(begin)
		//	log.Default().Printf("过去%v,  多节点总共完成%d笔交易\n", worker.pastTime, totalCounter)
	}
	a := rand.Int() % len(lst)
	b := rand.Int() % len(lst)
	trans(a, b)
}
