package main

import (
	"context"
	"fmt"

	"github.com/nathfavour/vibeauracle/pkg/vibe"
)

// HelloWorldVibe is a simple community-contributed vibe.
type HelloWorldVibe struct{}

func (p *HelloWorldVibe) Name() string {
	return "hello-world"
}

func (p *HelloWorldVibe) Description() string {
	return "A simple vibe that says hello to the community."
}

func (p *HelloWorldVibe) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok {
		name = "World"
	}
	return fmt.Sprintf("Hello, %s! Welcome to the vibeauracle ecosystem.", name), nil
}

// Ensure the vibe implements the interface
var _ vibe.Vibe = (*HelloWorldVibe)(nil)

func main() {
	fmt.Println("This is a vibe and not meant to be run directly.")
}

