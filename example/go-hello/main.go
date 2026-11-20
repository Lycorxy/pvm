package main

import (
	"fmt"
	"os"
	"runtime"
)

func greet(name string) string {
	return fmt.Sprintf("go-hello: %s (%s/%s)", name, runtime.GOOS, runtime.GOARCH)
}

func main() {
	name := "world"
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	fmt.Println(greet(name))
}
