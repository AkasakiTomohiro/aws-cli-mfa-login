package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/spf13/cobra"
)

var profile string
var outProfile string
var serialNumber string
var durationSeconds int32

// 引数で指定されたプロファイルから認証情報を取得
func loadAWSProfile(ctx context.Context) aws.Config {

	// AWS Configから指定されたプロファイルの認証情報を取得
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	if err != nil {
		// プロファイルが存在しない場合はエラー
		log.Fatalln(err)
	}

	// MFAのシリアル番号が指定されていない場合はAWS Configから取得
	if serialNumber == "" {
		outCfg, err := config.LoadSharedConfigProfile(ctx, outProfile)
		if err != nil {
			log.Fatalln(err)
		}
		serialNumber = outCfg.MFASerial
		if serialNumber == "" {
			log.Fatalln("There is no value for mfa_serial in the AWS configure specified in the --out-profile argument. Please specify the --serial-number argument.")
		}
	}
	return cfg
}

// ユーザーからMFAの認証コードを入力してもらい、認証情報を取得
func getSessionToken(ctx context.Context, cfg aws.Config) *types.Credentials {

	var tokenCode string
	var resp *sts.GetSessionTokenOutput
	var err error

	for {

		// ユーザーからMFAのOTPを入力してもらう
		fmt.Print("input token code: ")
		fmt.Scan(&tokenCode)
		tokenCode = strings.Trim(strings.Trim(tokenCode, "\r"), "\n")

		// 取得したOTPを使ってSTSのセッショントークンを取得
		stsClient := sts.NewFromConfig(cfg)
		resp, err = stsClient.GetSessionToken(ctx, &sts.GetSessionTokenInput{
			SerialNumber:    &serialNumber,
			TokenCode:       &tokenCode,
			DurationSeconds: &durationSeconds,
		})
		if err != nil {
			log.Println(err)
			continue
		}
		break
	}
	localTZ := time.Now().Location()
	log.Print("STS Token Expiration Date: ", resp.Credentials.Expiration.In(localTZ))

	return resp.Credentials
}

// 認証情報をAWS Configに保存
func setSessionToken(credentials *types.Credentials) {
	exec.Command("aws", "configure", "set", "aws_access_key_id", *credentials.AccessKeyId, "--profile", outProfile).Output()
	exec.Command("aws", "configure", "set", "aws_secret_access_key", *credentials.SecretAccessKey, "--profile", outProfile).Output()
	exec.Command("aws", "configure", "set", "aws_session_token", *credentials.SessionToken, "--profile", outProfile).Output()
	exec.Command("aws", "configure", "set", "region", "ap-northeast-1", "--profile", outProfile).Output()
	exec.Command("aws", "configure", "set", "output", "json", "--profile", outProfile).Output()
	exec.Command("aws", "configure", "set", "mfa_serial", serialNumber, "--profile", outProfile).Output()
}

var rootCmd = &cobra.Command{
	Use:   "aws-cli-mfa-login",
	Short: "A tool for obtaining AWS CLI sessions using a virtual authenticator application.",
	Run: func(cmd *cobra.Command, args []string) {

		ctx := context.Background()
		cfg := loadAWSProfile(ctx)
		credentials := getSessionToken(ctx, cfg)
		setSessionToken(credentials)
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
	rootCmd.Flags().StringVarP(&serialNumber, "serial-number", "s", "", "MFA identifier. If not specified, the value of mfa_serial in AWS configure specified in the --out-profile argument is used.\nEx: arn:aws:iam::123456789012:mfa/user")
	rootCmd.Flags().StringVarP(&outProfile, "out-profile", "o", "default-sts", "AWS profile name to be used in STS")
	rootCmd.Flags().Int32VarP(&durationSeconds, "duration-seconds", "d", 3600, "The duration, in seconds, that the credentials should remain valid.")
}
