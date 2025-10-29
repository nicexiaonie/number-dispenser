package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/nicexiaonie/number-dispenser/internal/protocol"
)

func main() {
	// Connect to the server
	conn, err := net.Dial("tcp", "localhost:6380")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := protocol.NewReader(conn)
	writer := protocol.NewWriter(conn)

	fmt.Println("Connected to Number Dispenser Server")
	fmt.Println("Commands:")
	fmt.Println("  HSET <name> type <type> [length <len>] [starting <start>] [step <step>]")
	fmt.Println("  GET <name>")
	fmt.Println("  INFO <name>")
	fmt.Println("  DEL <name>")
	fmt.Println("  PING")
	fmt.Println("  QUIT")
	fmt.Println()

	// Example: Create dispensers
	examples := [][]string{
		{"HSET", "fahaoqi1", "type", "1", "length", "7"},
		{"HSET", "fahaoqi2", "type", "2", "length", "8", "starting", "10001000"},
		{"HSET", "fahaoqi3", "type", "3", "starting", "5", "step", "3"},
	}

	fmt.Println("=== Creating Example Dispensers ===")
	for _, cmd := range examples {
		if err := sendCommand(writer, cmd); err != nil {
			log.Printf("Failed to send command: %v", err)
			continue
		}

		resp, err := reader.ReadValue()
		if err != nil {
			log.Printf("Failed to read response: %v", err)
			continue
		}

		fmt.Printf("Command: %v\n", cmd)
		printResponse(resp)
		fmt.Println()
	}

	// Generate some numbers
	fmt.Println("=== Generating Numbers ===")
	dispensers := []string{"fahaoqi1", "fahaoqi2", "fahaoqi3"}
	for _, name := range dispensers {
		fmt.Printf("\nDispenser: %s\n", name)
		for i := 0; i < 5; i++ {
			if err := sendCommand(writer, []string{"GET", name}); err != nil {
				log.Printf("Failed to send command: %v", err)
				continue
			}

			resp, err := reader.ReadValue()
			if err != nil {
				log.Printf("Failed to read response: %v", err)
				continue
			}

			printResponse(resp)
		}
	}

	// Show dispenser info
	fmt.Println("\n=== Dispenser Information ===")
	for _, name := range dispensers {
		if err := sendCommand(writer, []string{"INFO", name}); err != nil {
			log.Printf("Failed to send command: %v", err)
			continue
		}

		resp, err := reader.ReadValue()
		if err != nil {
			log.Printf("Failed to read response: %v", err)
			continue
		}

		fmt.Printf("\nDispenser: %s\n", name)
		printResponse(resp)
	}

	// Interactive mode
	fmt.Println("\n=== Interactive Mode (type commands or 'exit' to quit) ===")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := scanner.Text()
		if input == "exit" {
			break
		}

		// Parse input into command arguments
		args := parseInput(input)
		if len(args) == 0 {
			continue
		}

		if err := sendCommand(writer, args); err != nil {
			log.Printf("Failed to send command: %v", err)
			continue
		}

		resp, err := reader.ReadValue()
		if err != nil {
			log.Printf("Failed to read response: %v", err)
			continue
		}

		printResponse(resp)
	}
}

func sendCommand(writer *protocol.Writer, args []string) error {
	vals := make([]protocol.Value, len(args))
	for i, arg := range args {
		vals[i] = protocol.Value{Type: protocol.BulkString, Bulk: arg}
	}

	return writer.WriteArray(vals)
}

func printResponse(val protocol.Value) {
	switch val.Type {
	case protocol.SimpleString:
		fmt.Printf("OK: %s\n", val.Str)
	case protocol.Error:
		fmt.Printf("ERROR: %s\n", val.Str)
	case protocol.Integer:
		fmt.Printf("Integer: %d\n", val.Num)
	case protocol.BulkString:
		fmt.Printf("Result: %s\n", val.Bulk)
	case protocol.Array:
		fmt.Println("Array:")
		for i, v := range val.Array {
			fmt.Printf("  [%d] ", i)
			printResponse(v)
		}
	}
}

func parseInput(input string) []string {
	var args []string
	var current string
	inQuote := false

	for _, ch := range input {
		if ch == '"' {
			inQuote = !inQuote
		} else if ch == ' ' && !inQuote {
			if current != "" {
				args = append(args, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}

	if current != "" {
		args = append(args, current)
	}

	return args
}
