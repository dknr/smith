package cmd

import (
	"fmt"
	"os"

	"smith/client"
	"smith/logging"
	"smith/server"

	"github.com/spf13/cobra"
)

var (
	listenAddr string
)

var rootCmd = &cobra.Command{
	Use:   "smith",
	Short: "smith - an LLM agent client/server",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the websocket server",
	Run: func(cmd *cobra.Command, args []string) {
		logger, cleanup := logging.Setup("smith")
		defer cleanup()

		if err := server.Serve(listenAddr, logger); err != nil {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	},
}

var sendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send a message to the server and print the response",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logger, cleanup := logging.Setup("smith")
		defer cleanup()

		message := args[0]
		if err := client.ConnectAndSend(listenAddr, message, logger); err != nil {
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

	serveCmd.Flags().StringVarP(&listenAddr, "addr", "a", "localhost:26856", "listen address")
	sendCmd.Flags().StringVarP(&listenAddr, "addr", "a", "localhost:26856", "server address")
}
