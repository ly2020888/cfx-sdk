package main

import "os"

func test() {
	// 要追加的文本内容
	text := "要追加的文本\n"

	// 打开文件以追加内容，如果文件不存在则创建
	file, err := os.OpenFile(".//log.txt", os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// 将文本写入文件
	_, err = file.WriteString(text)
	if err != nil {
		panic(err)
	}

	// 使用 ioutil 包的 AppendFile 函数也可以实现相同的功能
	//err = ioutil.WriteFile("文件路径", []byte(text), os.ModeAppend)
	//if err != nil {
	//    panic(err)
	//}
}
