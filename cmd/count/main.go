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

	iter.SeekGE(nil)
	for iter.Valid() {
		count++
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

func testConfigKeys(db *pebble.DB) error {
	fmt.Println("\nTesting config keys:")
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: []byte("ethereum-config-"),
		UpperBound: []byte("ethereum-config."), // '.' is the next character after '-'
	})
	if err != nil {
		return fmt.Errorf("create iterator: %w", err)
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		if !bytes.HasPrefix(iter.Key(), []byte("ethereum-config-")) {
			break
		}
		fmt.Printf("Found config key: %x\n", iter.Key())
		val, err := iter.ValueAndErr()
		if err != nil {
			fmt.Printf("Error reading value: %v\n", err)
			continue
		}
		fmt.Printf("Value: %x\n", val)
	}
	return iter.Error()
}

func testCommonPrefixes(db *pebble.DB) error {
	fmt.Println("\nTesting common prefixes:")
	prefixes := []string{
		"h", // header
		"r", // receipt
		"b", // body
		"n", // number
		"l", // lookup
		"ethereum-config-",
	}

	for _, prefix := range prefixes {
		fmt.Printf("\nTesting prefix: %s\n", prefix)
		iter, err := db.NewIter(&pebble.IterOptions{})
		if err != nil {
			return fmt.Errorf("create iterator: %w", err)
		}

		iter.SeekGE([]byte(prefix))
		if iter.Valid() && bytes.HasPrefix(iter.Key(), []byte(prefix)) {
			fmt.Printf("First key: %x\n", iter.Key())
			val, err := iter.ValueAndErr()
			if err != nil {
				fmt.Printf("Error reading value: %v\n", err)
			} else {
				if len(val) > 100 {
					fmt.Printf("Value (first 100 bytes): %x...\n", val[:100])
				} else {
					fmt.Printf("Value: %x\n", val)
				}
			}
		} else {
			fmt.Printf("No key found with prefix\n")
		}
		iter.Close()
	}
	return nil
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

	// 1. 先统计总key数
	count, err := countKeys(db)
	if err != nil {
		log.Printf("Error counting keys: %v", err)
	} else {
		fmt.Printf("\nTotal number of keys in the database: %d\n", count)
	}

	// 2. 测试所有config相关的key
	if err := testConfigKeys(db); err != nil {
		log.Printf("Error testing config keys: %v", err)
	}

	// 3. 测试常见前缀
	if err := testCommonPrefixes(db); err != nil {
		log.Printf("Error testing common prefixes: %v", err)
	}

	// 4. 最后尝试直接获取目标key
	configPrefix := []byte("ethereum-config-")
	storedHash := common.HexToHash("0x0233022796c4160f5998145f153974ba65376a83825294f7811f44034461652f")
	configKey := append(configPrefix, storedHash.Bytes()...)

	fmt.Printf("\nTrying to get specific config key: %x\n", configKey)
	data, c, err := db.Get(configKey)
	if err != nil {
		fmt.Printf("Error getting key: %v\n", err)
	} else {
		defer c.Close()
		fmt.Printf("Success! Value: %s\n", hex.EncodeToString(data))
	}
}
