/*
 * Copyright (c) 2023 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"

	. "github.com/massenz/fsm-cli/client"
)

var (
	// Release is set by the Makefile at build time
	Release string

	insecure   bool
	serverAddr string
	logLevel   string

	clientSvc *CliClient
)

func main() {
	// Human-friendly console logger by default.
	output := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
	zlog.Logger = zlog.Output(output)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		zlog.Error().Err(err).Msg("command failed")
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fsm-cli",
		Short: "CLI client for the FSM server",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Configure log level from flag on every invocation.
			level, err := zerolog.ParseLevel(strings.ToLower(logLevel))
			if err != nil {
				zlog.Warn().Str("log-level", logLevel).Msg("invalid log level, defaulting to info")
				level = zerolog.InfoLevel
			}
			zerolog.SetGlobalLevel(level)
			return nil
		},
	}

	cmd.PersistentFlags().BoolVar(&insecure, "insecure", false, "If set, TLS will be disabled (NOT recommended)")
	cmd.PersistentFlags().StringVar(&serverAddr, "addr", "localhost:7398", "The address (host:port) for the gRPC server")
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (trace, debug, info, warn, error)")

	cmd.AddCommand(newSendCmd(), newGetCmd(), newVersionCmd())

	return cmd
}

// ensureClient initializes the gRPC client once per process and performs a
// basic health check to verify that the server is reachable.
func ensureClient() (*CliClient, *emptypb.Empty, error) {
	// Reuse the existing client if we already connected once.
	if clientSvc != nil {
		return clientSvc, &emptypb.Empty{}, nil
	}

	zlog.Info().Str("addr", serverAddr).Msg("connecting to FSM server")
	c := NewClient(serverAddr, !insecure)
	if c == nil {
		zlog.Error().Str("addr", serverAddr).Msg("cannot create gRPC client")
		return nil, nil, fmt.Errorf("cannot connect to server at %s", serverAddr)
	}

	if _, err := c.Health(context.Background(), &emptypb.Empty{}); err != nil {
		zlog.Error().Err(err).Msg("health check failed")
		return nil, nil, fmt.Errorf("cannot connect to server: %w", err)
	}

	clientSvc = c
	return clientSvc, &emptypb.Empty{}, nil
}

func newSendCmd() *cobra.Command {
	return &cobra.Command{
		Use:   fmt.Sprintf("%s [path]", CmdSend),
		Short: "Send an entity (Configuration, FSM, or Event) from a YAML file or stdin",
		Long: "Send an entity to the FSM server. The entity is described as YAML and can be " +
			"a Configuration, FiniteStateMachine, or EventRequest. Use '--' as path to read from stdin.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, _, err := ensureClient()
			if err != nil {
				return err
			}
			path := args[0]
			zlog.Debug().Str("path", path).Msg("sending entity")
			return svc.Send(path)
		},
	}
}

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   fmt.Sprintf("%s [kind] [id]", CmdGet),
		Short: "Retrieve a Configuration or FSM and print it as YAML",
		Long: "Retrieve an entity from the FSM server and print its YAML representation. " +
			"The kind must match one of the supported API kinds, for example 'Configuration' or 'FiniteStateMachine'.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, _, err := ensureClient()
			if err != nil {
				return err
			}
			kind, id := args[0], args[1]
			zlog.Debug().Str("kind", kind).Str("id", id).Msg("retrieving entity")
			return svc.Get(kind, id)
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   CmdVersion,
		Short: "Print client release and connected server information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, _, err := ensureClient()
			if err != nil {
				return err
			}

			resp, err := svc.Health(context.Background(), &emptypb.Empty{})
			if err != nil {
				return err
			}

			fmt.Println("FSM CLI Client Rel.", Release)
			fmt.Printf("Connected to Server: %s at %s (%s)\n", resp.Release, serverAddr, resp.State)
			return nil
		},
	}
}
