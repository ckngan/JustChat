// Author: Haniel Martino
package main

import (
	"fmt"
)

// Main method to setup for server
func main() {

  fmt.Println("I am your server")
}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error ", err.Error())
		os.Exit(1)
	}
}
