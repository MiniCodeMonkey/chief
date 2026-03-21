package main

import "github.com/minicodemonkey/chief/cmd/chief/commands"

// Version is set at build time via ldflags
var Version = "dev"

func main() {
	commands.Execute(Version)
}
