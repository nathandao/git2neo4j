package main

import (
	"fmt"
	"log"
	"os"

	"repository"
)

func main() {
	path := os.Args[1]
	repository := Repository{path}
	cc, err := repository.GetInfo()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(cc)
}
