package main

import (
	stdcontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"os"
	signalpkg "os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	sdk "github.com/Conflux-Chain/go-conflux-sdk"
	"github.com/Conflux-Chain/go-conflux-sdk/cfxclient/bulk"
	"github.com/Conflux-Chain/go-conflux-sdk/types"
	"github.com/Conflux-Chain/go-conflux-sdk/types/cfxaddress"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	am        *PrivatekeyAccountManager
	TotalTime float64 = 0
)

const (
	KEYDIR string = "./keystore"

	// KEYDIR string = "./keytest"

// 21是燃油费 -> 0.000021CFX
)

type TestBed struct {
	conf    Config
	workers []Worker
	sinal   chan int
	counter int
	mu      sync.Mutex
	running bool
}

type allocationRequest struct {
	Nodes  *int `json:"nodes"`
	Amount *int `json:"amount"`
}

type startRequest struct {
	Time *float64 `json:"time"`
}

type latencyRequest struct {
	Count       *int `json:"count"`
	IntervalMs  *int `json:"interval_ms"`
	WorkerIndex *int `json:"worker_index"`
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
	// keysToEnsure := []string{
	// 	"dc780c521c32237b3e44a2f3796d1e0f7ca0fe47d78df321814ee4e8091b4e68",
	// 	"6b365b5101c63af32b2b65e40467491c885d2b307502719a8242afb2a61f1ab3",
	// 	"1c7c78b21fd752c5512b15d74ccc5d56b57d23146ba00be702fad07a71461cdc",
	// }
	// if err := ensurePrivateKeys(keysToEnsure, "hello"); err != nil {
	// 	log.Fatalf("导入私钥失败: %v", err)
	// }
	readAccountToAm(config.Numbers) //将文件夹内的账户读入，优先串行加载关键账户
	// all := am.List()
	// for idx, addr := range all {
	// 	log.Default().Printf("account[%d] id %s", idx, addr.MustGetCommonAddress())
	// }
	accountShards := shardAccounts(am.List(), config.Numbers)
	clientOpts := sdk.ClientOption{
		RetryCount:           3,
		RetryInterval:        2 * time.Second,
		RequestTimeout:       90 * time.Second,
		MaxConnectionPerHost: 1024,
	}
	for i := 0; i < config.Numbers; i++ {
		client, err := sdk.NewClient(config.Urls[i], clientOpts)
		if err != nil {
			log.Printf("failed to create client for %s: %v", config.Urls[i], err)
			continue
		}
		log.Default().Printf("%v", config.Urls[i])
		client.SetAccountManager(am)

		worker := Worker{
			address:  config.Urls[i],
			rate:     config.Rate,
			client:   client,
			sinal:    &tb.sinal,
			accounts: accountShards[i],
			froms:    make([]cfxaddress.Address, 0),
			tos:      make([]cfxaddress.Address, 0),
		}
		fmt.Println("now worker", i, "has accounts:", len(worker.accounts))
		worker.bulkSender = bulk.NewBulkSender(*client)
		tb.workers = append(tb.workers, worker)
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
	//挖矿节点有config.Numbers个，然后直接分发金额
	//一个账号初始化时转给100，那么每个账户得到的钱是：  节点数 * 100
	//fmt.Println(len(am.List()))
	// 	dc780c521c32237b3e44a2f3796d1e0f7ca0fe47d78df321814ee4e8091b4e68
	// 6b365b5101c63af32b2b65e40467491c885d2b307502719a8242afb2a61f1ab3
	// 1c7c78b21fd752c5512b15d74ccc5d56b57d23146ba00be702fad07a71461cdc

	// tb.start(config.Time)

	srv := startHTTPServer(&tb)
	waitForShutdown(srv)
}

func final_single_test(tb *TestBed) {
	const totalRuns = 1000
	const concurrency = 100
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	rand.Seed(time.Now().UnixNano())
	for i := 0; i < totalRuns; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			idx := 0
			if len(tb.workers) > 1 {
				idx = rand.Intn(2)
			}
			tb.workers[idx].randomtransfer()
		}()
	}
	wg.Wait()
}

func (tb *TestBed) start(timeLimit float64) {
	if timeLimit <= 0 {
		timeLimit = 1
	}
	duration := time.Duration(timeLimit * float64(time.Second))
	if duration <= 0 {
		duration = time.Second
	}
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), duration)
	defer cancel()
	runStart := time.Now()

	for i := 0; i < len(tb.workers); i++ {
		tb.workers[i].resetForRun()
		log.Default().Printf("worker %d started", i)
		go tb.workers[i].cfxCal(ctx, int(NewConfig().Peers))
	}

	totalTransactions := 0
	for {
		totalTransactions += <-tb.sinal // 收到wokrer结束信号
		tb.counter++
		if tb.counter >= len(tb.workers) {
			log.Default().Printf("全部工人已经执行完工作\n")
			elapsedSeconds := time.Since(runStart).Seconds()
			if elapsedSeconds <= 0 {
				elapsedSeconds = duration.Seconds()
			}
			log.Default().Printf("执行时间%f(目标%f), 总计交易数量%d,不合法交易数量:%d,Tps:%f",
				elapsedSeconds,
				duration.Seconds(),
				totalTransactions,
				invalidTransactions,
				float64(totalTransactions)/elapsedSeconds,
			)
			return
		}

	}

}

func shardAccounts(accounts []types.Address, shardCount int) [][]types.Address {
	if shardCount <= 0 {
		return nil
	}
	shards := make([][]types.Address, shardCount)
	if len(accounts) == 0 {
		return shards
	}
	for i, addr := range accounts {
		idx := i % shardCount
		shards[idx] = append(shards[idx], addr)
	}
	return shards
}

func readAccountToAm(serialCount int) {
	dir := KEYDIR // 指定目录的路径

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	start := time.Now()
	serialPaths := make([]string, 0, serialCount)
	parallelPaths := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		path := filepath.Join(dir, file.Name())
		if serialCount > 0 && len(serialPaths) < serialCount {
			serialPaths = append(serialPaths, path)
			continue
		}
		parallelPaths = append(parallelPaths, path)
	}

	for _, path := range serialPaths {
		if _, err := am.Import(path, "hello", "hello"); err != nil {
			log.Printf("failed to import %s: %v", path, err)
		}
	}

	if len(parallelPaths) == 0 {
		elapsed := time.Since(start)
		fmt.Printf("readAccountToAm 该段代码运行消耗的时间为：%s", elapsed)
		fmt.Println(len(am.List()))
		return
	}
	workers := runtime.NumCPU() / 2
	if workers < 1 {
		workers = 1
	}
	if workers > len(parallelPaths) {
		workers = len(parallelPaths)
	}
	if workers == 0 {
		workers = 1
	}
	jobs := make(chan string, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				if _, err := am.Import(path, "hello", "hello"); err != nil {
					log.Printf("failed to import %s: %v", path, err)
				}
			}
		}()
	}
	for _, path := range parallelPaths {
		jobs <- path
	}
	close(jobs)
	wg.Wait()
	elapsed := time.Since(start)
	fmt.Printf("readAccountToAm 该段代码运行消耗的时间为：%s", elapsed)
	fmt.Println(len(am.List()))

}

func ensurePrivateKeys(keys []string, passphrase string) error {
	if err := os.MkdirAll(KEYDIR, 0o700); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	ks := keystore.NewKeyStore(KEYDIR, keystore.StandardScryptN, keystore.StandardScryptP)
	for _, key := range keys {
		cleaned := strings.TrimSpace(key)
		if cleaned == "" {
			continue
		}
		cleaned = strings.TrimPrefix(cleaned, "0x")

		priv, err := crypto.HexToECDSA(cleaned)
		if err != nil {
			return fmt.Errorf("解析私钥失败: %w", err)
		}

		_, err = ks.ImportECDSA(priv, passphrase)
		if err != nil {
			if errors.Is(err, keystore.ErrAccountAlreadyExists) {
				continue
			}
			return fmt.Errorf("导入私钥失败: %w", err)
		}
	}

	return nil
}

func startHTTPServer(tb *TestBed) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		if len(tb.workers) == 0 {
			http.Error(w, "no workers available", http.StatusServiceUnavailable)
			return
		}

		payload := startRequest{}
		if r.Body != nil {
			defer r.Body.Close()
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&payload); err != nil {
				if !errors.Is(err, io.EOF) {
					http.Error(w, fmt.Sprintf("invalid JSON payload: %v", err), http.StatusBadRequest)
					return
				}
			}
		}

		timeLimit := tb.conf.Time
		if payload.Time != nil {
			timeLimit = *payload.Time
		}
		if timeLimit <= 0 {
			http.Error(w, "time must be greater than 0", http.StatusBadRequest)
			return
		}

		tb.mu.Lock()
		if tb.running {
			tb.mu.Unlock()
			http.Error(w, "test already running", http.StatusConflict)
			return
		}
		tb.counter = 0
		tb.running = true
		tb.mu.Unlock()

		log.Printf("start triggered via API with time=%f", timeLimit)

		if err := file.Truncate(0); err != nil {
			log.Printf("failed to truncate log file: %v", err)
		} else if _, err := file.Seek(0, 0); err != nil {
			log.Printf("failed to reset log file cursor: %v", err)
		}

		go func(limit float64) {
			tb.start(limit)
			tb.mu.Lock()
			tb.running = false
			tb.mu.Unlock()
		}(timeLimit)

		response := map[string]interface{}{
			"status": "started",
			"time":   timeLimit,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("start response encode error: %v", err)
		}
	})
	mux.HandleFunc("/allocation", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		if len(tb.workers) == 0 {
			http.Error(w, "no workers available", http.StatusServiceUnavailable)
			return
		}

		payload := allocationRequest{}
		if r.Body != nil {
			defer r.Body.Close()
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&payload); err != nil {
				if !errors.Is(err, io.EOF) {
					http.Error(w, fmt.Sprintf("invalid JSON payload: %v", err), http.StatusBadRequest)
					return
				}
			}
		}

		nodes := tb.conf.Numbers
		if payload.Nodes != nil {
			nodes = *payload.Nodes
		}
		if nodes <= 0 {
			http.Error(w, "nodes must be greater than 0", http.StatusBadRequest)
			return
		}

		amount := 50
		if payload.Amount != nil {
			amount = *payload.Amount
		}

		log.Printf("allocation triggered via API with nodes=%d amount=%d", nodes, amount)
		tb.workers[0].allocation(nodes, amount)

		response := map[string]interface{}{
			"status": "allocation completed",
			"nodes":  nodes,
			"amount": amount,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("allocation response encode error: %v", err)
		}
	})
	mux.HandleFunc("/latency", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		if len(tb.workers) == 0 {
			http.Error(w, "no workers available", http.StatusServiceUnavailable)
			return
		}

		payload := latencyRequest{}
		if r.Body != nil {
			defer r.Body.Close()
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&payload); err != nil {
				if !errors.Is(err, io.EOF) {
					http.Error(w, fmt.Sprintf("invalid JSON payload: %v", err), http.StatusBadRequest)
					return
				}
			}
		}

		count := 4
		if payload.Count != nil {
			count = *payload.Count
		}
		if count <= 0 {
			http.Error(w, "count must be greater than 0", http.StatusBadRequest)
			return
		}

		interval := 3 * time.Second
		if payload.IntervalMs != nil {
			if *payload.IntervalMs < 0 {
				http.Error(w, "interval_ms must be non-negative", http.StatusBadRequest)
				return
			}
			interval = time.Duration(*payload.IntervalMs) * time.Millisecond
		}

		workerIdx := 0
		randomWorker := false
		if payload.WorkerIndex != nil {
			workerIdx = *payload.WorkerIndex
		}
		if workerIdx == -1 {
			randomWorker = true
		} else if workerIdx < 0 || workerIdx >= len(tb.workers) {
			http.Error(w, fmt.Sprintf("worker_index must be between 0 and %d, or -1 for random", len(tb.workers)-1), http.StatusBadRequest)
			return
		}

		latencies := make([]float64, 0, count)
		usedWorkers := make([]int, 0, count)
		for i := 0; i < count; i++ {
			selectedIdx := workerIdx
			if randomWorker {
				selectedIdx = rand.Intn(len(tb.workers))
			}
			start := time.Now()
			tb.workers[selectedIdx].LatencyRandomTransfer()
			latency := time.Since(start)
			latencyMs := float64(latency) / float64(time.Millisecond)
			latencies = append(latencies, latencyMs)
			usedWorkers = append(usedWorkers, selectedIdx)
			log.Printf("latency test iteration %d worker=%d latency=%s", i, selectedIdx, latency)
			if i < count-1 && interval > 0 {
				time.Sleep(interval)
			}
		}

		average := 0.0
		for _, latency := range latencies {
			average += latency
		}
		average /= float64(len(latencies))

		response := map[string]interface{}{
			"status":             "completed",
			"worker_index":       workerIdx,
			"count":              count,
			"interval_ms":        interval / time.Millisecond,
			"latencies_ms":       latencies,
			"average_latency_ms": average,
			"used_workers":       usedWorkers,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("latency response encode error: %v", err)
		}
	})

	mux.HandleFunc("/single_transfer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		if len(tb.workers) == 0 {
			http.Error(w, "no workers available", http.StatusServiceUnavailable)
			return
		}

		type singleTransferRequest struct {
			FromIndex *int `json:"from_index"`
			ToIndex   *int `json:"to_index"`
			// FromAddress *string  `json:"from_address"`
			// ToAddress   *string  `json:"to_address"`
			AmountCFX *int64 `json:"amount_cfx"`
		}

		payload := singleTransferRequest{}
		if r.Body != nil {
			defer r.Body.Close()
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&payload); err != nil {
				if !errors.Is(err, io.EOF) {
					http.Error(w, fmt.Sprintf("invalid JSON payload: %v", err), http.StatusBadRequest)
					return
				}
			}
		}

		// 默认选择0 work_index工作
		worker := &tb.workers[0]

		// pick from address
		var from types.Address
		if payload.FromIndex != nil {
			pool := worker.accountPool()
			if *payload.FromIndex < 0 || *payload.FromIndex >= len(pool) {
				http.Error(w, "from_index out of range", http.StatusBadRequest)
				return
			}
			from = pool[*payload.FromIndex]
		} else {
			// default: use a random account from the worker's pool (or global list)
			pool := worker.accountPool()
			if len(pool) == 0 {
				http.Error(w, "no accounts available to use as from", http.StatusBadRequest)
				return
			}
			from = pool[rand.Intn(len(pool))]
		}

		// pick to address
		var to types.Address
		if payload.ToIndex != nil {
			all := am.List()
			if *payload.ToIndex < 0 || *payload.ToIndex >= len(all) {
				http.Error(w, "to_index out of range", http.StatusBadRequest)
				return
			}
			to = all[*payload.ToIndex]
		} else {
			// default: random global account (ensure not same as from if possible)
			all := am.List()
			// try to pick different address than 'from'
			for i := 0; i < 5; i++ {
				cand := all[rand.Intn(len(all))]
				if cand.String() != from.String() {
					to = cand
					break
				}
			}
		}
		// amount in CFX -> convert to internal units used by worker
		amountCFX := int64(1)
		if payload.AmountCFX != nil {
			amountCFX = *payload.AmountCFX
			if amountCFX <= 0 {
				http.Error(w, "amount_cfx must be positive", http.StatusBadRequest)
				return
			}
		}
		// convert to micro units then to base unit as worker.allocation does
		// singleTransfer is a *hexutil.Big, use big.Int math to multiply
		amtBig := new(big.Int).SetInt64(amountCFX)
		singleBig := (*big.Int)(singleTransfer)
		prod := new(big.Int).Mul(amtBig, singleBig)
		value := (*hexutil.Big)(prod)

		// perform the transfer (single tx)
		worker.transfer(from, to, value)

		response := map[string]interface{}{
			"status":     "submitted",
			"from":       from.String(),
			"to":         to.String(),
			"amount_cfx": amountCFX,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("single_transfer response encode error: %v", err)
		}
	})
	mux.HandleFunc("/balances", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		if len(tb.workers) == 0 {
			http.Error(w, "no workers available", http.StatusServiceUnavailable)
			return
		}

		balances := tb.workers[0].GetAllBalance()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(balances); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		log.Printf("HTTP API listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http server stopped: %v", err)
		}
	}()

	return srv
}

func waitForShutdown(srv *http.Server) {
	stop := make(chan os.Signal, 1)
	signalpkg.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("error shutting down server: %v", err)
	} else {
		log.Println("server shut down gracefully")
	}
}
