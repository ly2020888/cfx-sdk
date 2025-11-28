package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/accounts/keystore"
)

const defaultKeyDir = "./tmp_keystore"

func main() {
	count := 50
	keyDir := defaultKeyDir
	passphrase := "hello"
	flag.Parse()

	absDir, err := filepath.Abs(keyDir)
	if err != nil {
		log.Fatalf("resolve keydir: %v", err)
	}

	if err := os.MkdirAll(absDir, 0o700); err != nil {
		log.Fatalf("create keydir: %v", err)
	}

	ks := keystore.NewKeyStore(absDir, keystore.StandardScryptN, keystore.StandardScryptP)

	fmt.Printf("Generating %d accounts into %s\n", count, absDir)
	for i := 0; i < count; i++ {
		account, err := ks.NewAccount(passphrase)
		if err != nil {
			log.Fatalf("create account %d failed: %v", i+1, err)
		}
		fmt.Printf("[%03d] %s\n", i+1, account.Address.Hex())
	}
	fmt.Println("Done. Remember to fund the new accounts before use.")
}
