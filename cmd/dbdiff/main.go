package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"

	"github.com/cockroachdb/pebble"
)

func compareDBs(db1, db2 *pebble.DB) error {
	iter1, err := db1.NewIter(&pebble.IterOptions{})
	if err != nil {
		return fmt.Errorf("create iter1: %v", err)
	}
	defer iter1.Close()

	iter2, err := db2.NewIter(&pebble.IterOptions{})
	if err != nil {
		return fmt.Errorf("create iter2: %v", err)
	}
	defer iter2.Close()

	valid1 := iter1.SeekGE(nil)
	valid2 := iter2.SeekGE(nil)

	for valid1 && valid2 {
		key1 := iter1.Key()
		key2 := iter2.Key()

		cmp := bytes.Compare(key1, key2)
		switch {
		case cmp == 0:
			if !bytes.Equal(iter1.Value(), iter2.Value()) {
				return fmt.Errorf("different value for key: %x\nDB1 value: %x\nDB2 value: %x",
					key1, iter1.Value(), iter2.Value())
			}
			valid1 = iter1.Next()
			valid2 = iter2.Next()

		case cmp < 0:
			return fmt.Errorf("key only in DB1: %x", key1)

		case cmp > 0:
			return fmt.Errorf("key only in DB2: %x", key2)
		}
	}

	if valid1 {
		return fmt.Errorf("key only in DB1: %x", iter1.Key())
	}

	if valid2 {
		return fmt.Errorf("key only in DB2: %x", iter2.Key())
	}

	return nil
}

func main() {
	db1Path := flag.String("db1", "", "Path to first Pebble database")
	db2Path := flag.String("db2", "", "Path to second Pebble database")
	flag.Parse()

	if *db1Path == "" || *db2Path == "" {
		log.Fatal("Both database paths must be specified")
	}

	db1, err := pebble.Open(*db1Path, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open first database: %v", err)
	}
	defer db1.Close()

	db2, err := pebble.Open(*db2Path, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open second database: %v", err)
	}
	defer db2.Close()

	if err := compareDBs(db1, db2); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Databases are identical")
}
