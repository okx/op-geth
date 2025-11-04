package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/cockroachdb/pebble"
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
}
