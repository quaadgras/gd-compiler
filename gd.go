package main

import (
	"fmt"
	"os"

	_ "embed"

	"github.com/quaadgras/gd-compiler/internal/target/c99"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: gd-compiler [build/run]")
		return
	}
	switch os.Args[1] {
	case "build":
		if err := build("."); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "test":
		if err := test("."); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "run":
		if err := run("."); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	default:
		fmt.Println("Usage: gd [build/run]")
		os.Exit(1)
	}
}

func build(pkg string) error {
	return c99.Build(pkg, false)
}

func test(pkg string) error {
	if err := c99.Build(pkg, true); err != nil {
		return err
	}
	return nil
}

func run(pkg string) error {
	if err := build(pkg); err != nil {
		return err
	}
	return nil
}
