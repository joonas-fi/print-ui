package main

// Util for generating identifiers

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/function61/gokit/app/cli"
	"github.com/spf13/cobra"
)

func genIdsEntrypoint() *cobra.Command {
	amount := 1

	cmd := &cobra.Command{
		Use:   "gen-ids",
		Short: "Generate identifiers",
		Args:  cobra.NoArgs,
		Run: cli.RunnerNoArgs(func(_ context.Context, _ *log.Logger) error {
			if amount < 0 {
				return errors.New("amount cannot be negative")
			}

			for i := 0; i < amount; i++ {
				fmt.Println(defaultIdentifierBuilder())
			}

			return nil
		}),
	}

	cmd.Flags().IntVarP(&amount, "amount", "a", amount, "Amount of identifiers to generate")

	return cmd
}
