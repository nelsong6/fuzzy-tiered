package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"

	"github.com/nelsong6/fzt/core"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: fzt <query>")
		os.Exit(1)
	}
	query := os.Args[1]

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}

	type scored struct {
		line  string
		score core.TieredScore
	}
	var results []scored
	for _, line := range lines {
		ts, _ := core.ScoreItem([]string{line}, query, nil, nil)
		if ts.Total() > 0 {
			results = append(results, scored{line, ts})
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[j].score.Less(results[i].score)
	})

	for _, r := range results {
		fmt.Println(r.line)
	}
}
