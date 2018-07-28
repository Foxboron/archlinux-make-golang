package main

import (
	"fmt"
	"os"
)

func main() {

	args := os.Args[1:]

	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "help":
		fmt.Println("lol")
	case "estimate":
	case "make":
		execMake(args[1:])
	default:
		execMake(args[1:])
	}

}
