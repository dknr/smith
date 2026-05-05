package client

import (
	"github.com/chzyer/readline"
)

// buildCompleter returns a prefix completer for slash commands.
func buildCompleter() readline.AutoCompleter {
	items := []readline.PrefixCompleterInterface{
		readline.PcItem("/quit"),
		readline.PcItem("/compact"),
		readline.PcItem("/mode"),
		readline.PcItem("/help"),
	}
	return readline.NewPrefixCompleter(items...)
}