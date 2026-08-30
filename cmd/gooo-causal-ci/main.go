package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "gooo-causal-ci: implementation is introduced in the pull request")
	os.Exit(2)
}
