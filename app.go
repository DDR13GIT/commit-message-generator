package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	ctx := context.Background()
	var err error
	if len(os.Args) == 2 && os.Args[1] == "keys" {
		err = runKeyUI(ctx)
	} else {
		client := &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		err = run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr,
			func(ctx context.Context, p provider, key, model, diff string) (string, error) {
				return generateSubject(ctx, client, p, key, model, diff)
			})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "cmsg:", err)
		os.Exit(1)
	}
}
