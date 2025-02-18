package cmd

import (
	"context"
	"os"

	"artan.jp/aws-cli-mfa-login/internal"
	"github.com/spf13/cobra"
)

var profile string
var outProfile string
var durationSeconds int32
var updateSerialNumber bool

var rootCmd = &cobra.Command{
	Use:   "aws-cli-mfa-login",
	Short: "A tool for obtaining AWS CLI sessions using a virtual authenticator application.",
	Run: func(cmd *cobra.Command, args []string) {

		ctx := context.Background()
		cfg, selectSerialNumber := internal.LoadAWSProfile(ctx, profile, outProfile, updateSerialNumber)
		credentials := internal.GetSessionToken(ctx, cfg, selectSerialNumber, durationSeconds)
		internal.SetSessionToken(outProfile, selectSerialNumber, credentials)
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&profile, "profile", "p", "default", "AWS profile name")
	rootCmd.Flags().StringVarP(&outProfile, "out-profile", "o", "default-sts", "AWS profile name to be used in STS")
	rootCmd.Flags().Int32VarP(&durationSeconds, "duration-seconds", "d", 3600, "The duration, in seconds, that the credentials should remain valid.")
	rootCmd.Flags().BoolVarP(&updateSerialNumber, "update-serial-number", "u", false, "Update the MFA serial number")
}
