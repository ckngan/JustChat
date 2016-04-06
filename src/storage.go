// Author: Haniel Martino
package main

import (
	"fmt"
	"log"
	"os"
)

// Main method to setup for storage
func main() {

	fmt.Println("I am your storage")
}

func checkError(err error) {
	if err != nil {
		log.Fatal(os.Stderr, "Error ", err.Error())
		os.Exit(1)
	}
}
