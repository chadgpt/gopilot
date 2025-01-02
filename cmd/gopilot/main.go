package main

import (
	"os"

	"github.com/chadgpt/gopilot"
)

func main() {
	err := gopilot.Run(os.Args[1:])
	if err != nil {
		panic(err)
	}
}
