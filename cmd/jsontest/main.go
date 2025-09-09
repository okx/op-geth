// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bytedance/sonic"
	"github.com/ethereum/go-ethereum/core"
)

func main() {
	var (
		filename   = flag.String("file", "", "Path to genesis.json file")
		iterations = flag.Int("iterations", 1, "Number of iterations to run for benchmarking")
	)
	flag.Parse()

	if *filename == "" {
		fmt.Fprintf(os.Stderr, "Error: -file flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Read the genesis.json file
	data, err := os.ReadFile(*filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", *filename, err)
		os.Exit(1)
	}

	fmt.Printf("JSON Performance Comparison\n")
	fmt.Printf("===========================\n")
	fmt.Printf("File: %s\n", *filename)
	fmt.Printf("File size: %d bytes\n", len(data))
	fmt.Printf("Iterations: %d\n\n", *iterations)

	// Test standard library JSON
	stdUnmarshalTime, stdMarshalTime, err := benchmarkStdJSON(data, *iterations)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error benchmarking standard JSON: %v\n", err)
		os.Exit(1)
	}

	// Test Sonic JSON
	sonicUnmarshalTime, sonicMarshalTime, err := benchmarkSonicJSON(data, *iterations)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error benchmarking Sonic JSON: %v\n", err)
		os.Exit(1)
	}

	// Print results
	fmt.Printf("Results:\n")
	fmt.Printf("--------\n")
	fmt.Printf("Standard Library JSON:\n")
	fmt.Printf("  Unmarshal: %v (avg: %v per operation)\n", stdUnmarshalTime, stdUnmarshalTime/time.Duration(*iterations))
	fmt.Printf("  Marshal:   %v (avg: %v per operation)\n", stdMarshalTime, stdMarshalTime/time.Duration(*iterations))
	fmt.Printf("  Total:     %v\n\n", stdUnmarshalTime+stdMarshalTime)

	fmt.Printf("Sonic JSON:\n")
	fmt.Printf("  Unmarshal: %v (avg: %v per operation)\n", sonicUnmarshalTime, sonicUnmarshalTime/time.Duration(*iterations))
	fmt.Printf("  Marshal:   %v (avg: %v per operation)\n", sonicMarshalTime, sonicMarshalTime/time.Duration(*iterations))
	fmt.Printf("  Total:     %v\n\n", sonicUnmarshalTime+sonicMarshalTime)

	// Calculate speedup
	stdTotal := stdUnmarshalTime + stdMarshalTime
	sonicTotal := sonicUnmarshalTime + sonicMarshalTime

	if sonicTotal > 0 {
		speedup := float64(stdTotal) / float64(sonicTotal)
		fmt.Printf("Performance Comparison:\n")
		fmt.Printf("----------------------\n")
		fmt.Printf("Sonic is %.2fx faster than standard library\n", speedup)
		fmt.Printf("Time saved: %v (%.1f%% improvement)\n",
			stdTotal-sonicTotal,
			float64(stdTotal-sonicTotal)/float64(stdTotal)*100)
	}
}

func benchmarkStdJSON(data []byte, iterations int) (time.Duration, time.Duration, error) {
	var genesis core.Genesis

	// Benchmark unmarshal
	unmarshalStart := time.Now()
	for i := 0; i < iterations; i++ {
		if err := json.Unmarshal(data, &genesis); err != nil {
			return 0, 0, err
		}
	}
	unmarshalTime := time.Since(unmarshalStart)

	// Benchmark marshal
	marshalStart := time.Now()
	for i := 0; i < iterations; i++ {
		if _, err := json.Marshal(&genesis); err != nil {
			return 0, 0, err
		}
	}
	marshalTime := time.Since(marshalStart)

	return unmarshalTime, marshalTime, nil
}

func benchmarkSonicJSON(data []byte, iterations int) (time.Duration, time.Duration, error) {
	var genesis core.Genesis

	// Benchmark unmarshal
	unmarshalStart := time.Now()
	for i := 0; i < iterations; i++ {
		if err := sonic.Unmarshal(data, &genesis); err != nil {
			return 0, 0, err
		}
	}
	unmarshalTime := time.Since(unmarshalStart)

	// Benchmark marshal
	marshalStart := time.Now()
	for i := 0; i < iterations; i++ {
		if _, err := sonic.Marshal(&genesis); err != nil {
			return 0, 0, err
		}
	}
	marshalTime := time.Since(marshalStart)

	return unmarshalTime, marshalTime, nil
}
