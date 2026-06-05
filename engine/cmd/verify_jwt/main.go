// Quick CLI to verify a JWT signed by tools/issue_jwt.py.
// Used by the launch checklist for end-to-end auth confirmation.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/anishmah100/agent_sim/engine/internal/security"
)

func main() {
	secret := flag.String("secret", "", "HS256 secret")
	flag.Parse()
	if flag.NArg() < 1 || *secret == "" {
		fmt.Fprintln(os.Stderr, "usage: verify_jwt -secret <s> <token>")
		os.Exit(2)
	}
	claims, err := security.VerifyJWT(flag.Arg(0), []byte(*secret))
	if err != nil {
		fmt.Println("FAIL:", err)
		os.Exit(1)
	}
	fmt.Printf("OK sub=%v exp=%v\n", claims["sub"], claims["exp"])
}
