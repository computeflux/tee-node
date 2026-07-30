package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/computeflux/tee-provider/ext/actives"
)

func main() {
	to := flag.String("to", "", "recipient address (required)")
	amount := flag.Int64("amount", 0, "token amount in whole units, before 18 decimals (required)")
	config := flag.String("config", "flux_config.json", "path to flux_config.json")
	flag.Parse()

	if *to == "" || *amount <= 0 {
		fmt.Fprintf(os.Stderr, "Usage: go run ./cmd/mint -to=0x... -amount=1000\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if err := actives.InitFluxChain(*config); err != nil {
		fmt.Fprintf(os.Stderr, "init flux chain: %v\n", err)
		os.Exit(1)
	}

	if err := actives.MintToken(*to, *amount); err != nil {
		fmt.Fprintf(os.Stderr, "mint failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Done.")
}
