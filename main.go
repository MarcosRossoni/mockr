package main

// Em Go, todo arquivo começa com "package".
// O package "main" é especial: é o ponto de entrada do programa.
// A função main() é equivalente ao public static void main() do Java.

import (
	"fmt"
	"os"

	"mockr/cmd"
)

func main() {
	if err := cmd.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}
