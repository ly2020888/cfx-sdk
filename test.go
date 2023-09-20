package main

import (
	"fmt"
	"time"
)

func test() {
	C := time.After(time.Duration(20) * time.Second)
	fmt.Println(C)
	begin := time.Now()
	time.Sleep(time.Second * 3)
	elapsed := time.Since(begin)
	fmt.Println(elapsed.Seconds())
}
