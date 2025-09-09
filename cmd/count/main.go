package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/common"
)

func countKeys(db *pebble.DB) (int64, error) {
	var count int64
	start := time.Now()
	lastReport := start

	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		return 0, err
	}
	defer iter.Close()

	configPrefix := []byte("ethereum-config-")

	iter.SeekGE(nil)
	for iter.Valid() {
		count++
		if bytes.HasPrefix(iter.Key(), configPrefix) {
			fmt.Printf("key: %x\n", iter.Key())
		}
		if count%1000000 == 0 {
			now := time.Now()
			elapsed := now.Sub(lastReport)
			speed := float64(1000000) / elapsed.Seconds()
			totalElapsed := now.Sub(start)
			fmt.Printf("Processed %d keys, speed: %.2f keys/s, elapsed: %v\n", count, speed, totalElapsed)
			lastReport = now
			runtime.GC()
		}
		iter.Next()
	}

	if err := iter.Error(); err != nil {
		return 0, fmt.Errorf("iteration error: %w", err)
	}

	elapsed := time.Since(start)
	speed := float64(count) / elapsed.Seconds()
	fmt.Printf("\nFinal statistics:\n")
	fmt.Printf("Total keys: %d\n", count)
	fmt.Printf("Total time: %v\n", elapsed)
	fmt.Printf("Average speed: %.2f keys/s\n", speed)

	return count, nil
}

func main() {
	dbPath := flag.String("db", "", "Path to the Pebble database")
	flag.Parse()

	if *dbPath == "" {
		log.Fatal("Database path must be specified using -db flag")
	}

	opts := &pebble.Options{
		Cache:      pebble.NewCache(1 << 30),
		DisableWAL: true,
	}

	db, err := pebble.Open(*dbPath, opts)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	count, err := countKeys(db)
	if err != nil {
		log.Fatalf("Error counting keys: %v", err)
	}
	fmt.Printf("\nTotal number of keys in the database: %d\n", count)

	configPrefix := []byte("ethereum-config-")
	storedHash := common.HexToHash("0x0233022796c4160f5998145f153974ba65376a83825294f7811f44034461652f")
	configKey := append(configPrefix, storedHash.Bytes()...)

	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		panic(err)
	}
	defer iter.Close()

	if iter.SeekGE(configKey) {
		key := iter.Key()
		fmt.Printf("Found key: %s\n", hex.EncodeToString(key[:]))
		value, err := iter.ValueAndErr()
		if err != nil {
			panic(err)
		}
		fmt.Printf("Value: %s\n", hex.EncodeToString(value[:]))
	} else {
		fmt.Printf("Key not found\n")
	}

	data, c, err := db.Get(configKey)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	fmt.Printf("%s\n", hex.EncodeToString(data))
	return
}
