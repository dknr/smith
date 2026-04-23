package cmd

import (
	"fmt"
	"os"

	"smith/client"
	"smith/config"
	"smith/llm"
	"smith/logging"
	"smith/memory"
	"smith/server"
	"smith/session"

	"github.com/spf13/cobra"
)

var (
	listenAddr string
	debug      bool
	verbose    bool
	debugTurns string
	version    = "dev"
)

var rootCmd = &cobra.Command{
	Use:     "smith",
	Short:   "smith - an LLM agent client/server",
	Version: version,
}

var serveCmd = &cobra.Command{
	Use:   "serve [db_path]",
	Short: "Start the websocket server",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		serverLogger, networkLogger, cleanup := logging.SetupServer(debug)
		defer cleanup()

		cfg, err := config.Load()
		if err != nil {
			serverLogger.Error("config error", "error", err)
			os.Exit(1)
		}

		dbPath := ":memory:"
		if len(args) > 0 {
			dbPath = args[0]
		}

		sess, err := session.NewWithDB(dbPath)
		if err != nil {
			serverLogger.Error("session error", "error", err)
			os.Exit(1)
		}
		defer sess.Close()

		memStore, err := memory.NewWithDB(dbPath)
		if err != nil {
			serverLogger.Error("memory store error", "error", err)
			os.Exit(1)
		}
		defer memStore.Close()

	turnPath := "log/turns"
		if debugTurns != "" {
			turnPath = debugTurns
		}

		turnLogger, err := llm.NewTurnLogger(turnPath)
		if err != nil {
			serverLogger.Error("turn logger error", "error", err)
			os.Exit(1)
		}

		if err := server.Serve(listenAddr, cfg, networkLogger, sess, memStore, serverLogger, turnLogger); err != nil {
			serverLogger.Error("server error", "error", err)
			os.Exit(1)
		}
	},
}

var sendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send a message to the server and print the response",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logger, cleanup := logging.SetupClient("send", debug)
		defer cleanup()

		if err := client.Send(listenAddr, args[0], logger, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	},
}

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session with the server",
	Run: func(cmd *cobra.Command, args []string) {
		logger, cleanup := logging.SetupClient("chat", debug)
		defer cleanup()

		term, err := client.NewTerminal("> ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer term.Close()

		if err := client.Chat(listenAddr, logger, term); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(sendCmd)
	rootCmd.AddCommand(chatCmd)

	rootCmd.PersistentFlags().StringVarP(&listenAddr, "addr", "a", "localhost:26856", "server address")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug logging to log/smith-*.log")
	serveCmd.Flags().StringVar(&debugTurns, "debug-turns", "", "enable turn request/response logging (default: log/turns)")
	sendCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show tool calls and stats in send mode")
}
