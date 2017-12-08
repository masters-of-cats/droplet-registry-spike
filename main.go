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
	appsPath := flag.String("apps-path", "", "to be replaced by Cloud Foundry blobstore access")
	flag.Parse()

	if *store == "" {
		panic("please set --store")
	}
	if *rootfsPath == "" {
		panic("please set --rootfs-path")
	}
	if *appsPath == "" {
		panic("please set --apps-path")
	}

	storeMgr := &storeManager{path: *store, appsPath: *appsPath, logger: logger}
	storeMgr.importRootfs(*rootfsPath)

	NewAPI(*listenAddress, storeMgr).ListenAndServe()
}

func must(action string, err error) {
	if err != nil {
		fmt.Printf("error %s: %s", action, err)
		os.Exit(1)
	}
}
