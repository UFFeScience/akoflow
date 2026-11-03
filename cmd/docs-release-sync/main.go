package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	output := flag.String("output", "docs/src/data/releases.generated.json", "Public release history snapshot")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 20 * time.Second}
	if err := synchronize(ctx, client, githubReleases, os.Getenv("GITHUB_TOKEN"), *output, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
