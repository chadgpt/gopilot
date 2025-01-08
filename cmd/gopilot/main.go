package main

import (
	"log"
	"os"

	"github.com/chadgpt/gopilot"
)

func main() {
	if os.Getenv("GHU_TOKEN") == "" {
		log.Fatalln("GHU_TOKEN is not set")
	}
	err := gopilot.Run(os.Args[1:])
	if err != nil {
		log.Fatalln(err)
	}
}
