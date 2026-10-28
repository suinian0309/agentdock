//go:build !windows

package main

import "fmt"

func main() { fmt.Println("agentdock-service is only supported on Windows") }
