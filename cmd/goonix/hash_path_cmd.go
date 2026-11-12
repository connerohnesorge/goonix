package main

import (
	"fmt"
	"os"

	"github.com/connerohnesorge/goonix/store/local"
)

// cmdHashPath prints the SHA-256 NAR hash of each path in Nix base32, as
// nix-hash --type sha256 --base32 does, with the sha256: prefix narinfo uses.
// On Windows the NAR reads the executable stream and case-hack names, so this
// is the hash a Windows store registers.
func cmdHashPath(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "hash-path expects one or more paths")
		os.Exit(2)
	}
	for _, p := range args {
		h, _, err := local.NarHashAndSize(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("sha256:%s\n", h)
	}
}
