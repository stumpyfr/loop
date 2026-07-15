package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "loop: %v\n", err)
		os.Exit(1)
	}
}
