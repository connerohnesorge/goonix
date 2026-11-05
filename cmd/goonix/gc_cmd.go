package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/connerohnesorge/goonix/store/local"
)

func cmdGC(args []string) {
	fs := flag.NewFlagSet("gc", flag.ExitOnError)
	storeFlag := fs.String("store", "", "physical store root (contains store/ and gcroots/)")
	_ = fs.Parse(args)
	if *storeFlag == "" {
		fmt.Fprintln(os.Stderr, "gc requires --store")
		os.Exit(2)
	}
	st, err := local.OpenStore(*storeFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer st.Close()
	deleted, err := st.CollectGarbage()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, p := range deleted {
		fmt.Println(p)
	}
}
