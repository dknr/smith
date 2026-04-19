package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"smith/client"
	"smith/config"
	"smith/logging"
	"smith/memory"
	"smith/server"
	"smith/session"

	"github.com/spf13/cobra"
)

var (
	listenAddr   string
	logProtocol  bool
	verbose      bool
	version      = "dev"
)

var rootCmd = &cobra.Command{
	Use:     "smith",
	Short:   "smith - an LLM agent client/server",
	Version: version,
}

func runWithLogger(programName string, fn func(logger *slog.Logger) error) {
	logger, cleanup := logging.Setup(programName)
	defer cleanup()

	if err := fn(logger); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

var serveCmd = &cobra.Command{
	Use:   "serve [db_path]",
	Short: "Start the websocket server",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runWithLogger("smith", func(logger *slog.Logger) error {
			cfg, err := config.Load()
			if err != nil {
				logger.Error("config error", "error", err)
				return err
			}

			dbPath := ":memory:"
			if len(args) > 0 {
				dbPath = args[0]
			}

			var protoLogger *slog.Logger
			if logProtocol {
				f, err := os.OpenFile("smith-protocol.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					logger.Error("protocol log error", "error", err)
					return err
				}
				protoLogger = slog.New(slog.NewJSONHandler(f, nil))
				defer f.Close()
			}

			sess, err := session.NewWithDB(dbPath)
			if err != nil {
				logger.Error("session error", "error", err)
				return err
			}
			defer sess.Close()

			memStore, err := memory.NewWithDB(dbPath)
			if err != nil {
				logger.Error("memory store error", "error", err)
				return err
			}
			defer memStore.Close()

			return server.Serve(listenAddr, cfg, protoLogger, sess, memStore, logger)
		})
	},
}

var sendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send a message to the server and print the response",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runWithLogger("smith", func(logger *slog.Logger) error {
			return client.Send(listenAddr, args[0], logger, verbose)
		})
	},
}

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session with the server",
	Run: func(cmd *cobra.Command, args []string) {
		runWithLogger("smith", func(logger *slog.Logger) error {
			return client.Chat(listenAddr, logger)
		})
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
	serveCmd.Flags().BoolVar(&logProtocol, "log-protocol", false, "log protocol traffic to smith-protocol.log")
	sendCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show tool calls and stats in send mode")
}
