package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	logger := log.New(os.Stdout, "[spikistry] ", log.LstdFlags)
	logger.Println("a very spiky registry")

	store := flag.String("store", "", "store")
	listenAddress := flag.String("listen-address", "", "listen-address")
	rootfsPath := flag.String("rootfs-path", "", "rootfs")
	capiURL := flag.String("capi-url", "", "capi-url")
	capiAuthToken := flag.String("capi-authtoken", "", "capi-authtoken")
	flag.Parse()

	if *store == "" {
		panic("please set --store")
	}
	if *rootfsPath == "" {
		panic("please set --rootfs-path")
	}
	if *capiURL == "" {
		panic("please set --capi-url")
	}
	if *capiAuthToken == "" {
		panic("please set --capi-authtoken")
	}

	storeMgr := &storeManager{
		path:          *store,
		capiURL:       *capiURL,
		capiAuthToken: *capiAuthToken,
		logger:        logger,
	}
	storeMgr.importRootfs(*rootfsPath)

	NewAPI(*listenAddress, storeMgr).ListenAndServe()
}

func must(action string, err error) {
	if err != nil {
		fmt.Printf("error %s: %s", action, err)
		os.Exit(1)
	}
}
