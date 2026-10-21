package cmd

import (
	"github.com/spf13/cobra"

	"pxon/internal/ui"
)

// reportedError prevents Execute from emitting a second JSON document.
type reportedError struct{ error }

func (e reportedError) Unwrap() error { return e.error }

func writeJSONResult(cmd *cobra.Command, result any, resultErr error) error {
	if err := ui.WriteJSON(cmd.OutOrStdout(), result); err != nil {
		return err
	}
	if resultErr != nil {
		return reportedError{resultErr}
	}
	return nil
}
