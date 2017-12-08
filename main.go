package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	store := flag.String("store", "", "store")
	listenAddress := flag.String("listen-address", "", "listen-address")
	flag.Parse()

	NewAPI(*listenAddress, *store).ListenAndServe()
}

func must(action string, err error) {
	if err != nil {
		fmt.Printf("error %s: %s", action, err)
		os.Exit(1)
	}
}
