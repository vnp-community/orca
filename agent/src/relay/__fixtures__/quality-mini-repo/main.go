package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Printf("%d\n", "string") // vet error
	os.Open("nonexistent") // errcheck
}
