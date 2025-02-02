package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/spf13/cobra"
)

var profile string
var outProfile string
var serialNumber string
var durationSeconds int32

// rootCmd は、サブコマンドなしで呼び出された場合の基本コマンドを表します。
var rootCmd = &cobra.Command{
	Use:   "aws-cli-mfa-login",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// 次の行のコメントを解除すると、ベアアプリケーションに関連するアクションがある場合に実行されます
	Run: func(cmd *cobra.Command, args []string) {

		// AWS Configから指定されたプロファイルの認証情報を取得
		cfg, err := config.LoadDefaultConfig(context.Background(), config.WithSharedConfigProfile(profile))
		if err != nil {
			log.Fatalln(err)
			return
		}

		// MFAのシリアル番号が指定されていない場合はAWS Configから取得
		if serialNumber == "" {
			outCfg, err := config.LoadSharedConfigProfile(context.Background(), outProfile)
			if err != nil {
				log.Fatalln(err)
				return
			}
			serialNumber = outCfg.MFASerial
			if serialNumber == "" {
				log.Fatalln("There is no value for mfa_serial in the AWS configure specified in the --out-profile argument. Please specify the --serial-number argument.")
				return
			}
		}

		// ユーザーからMFAのOTPを入力してもらう
		var tokenCode string
		var resp *sts.GetSessionTokenOutput

		for {

			fmt.Print("input code: ")
			fmt.Scan(&tokenCode)
			tokenCode = strings.Trim(strings.Trim(tokenCode, "\r"), "\n")

			// 取得したOTPを使ってSTSのセッショントークンを取得
			stsClient := sts.NewFromConfig(cfg)
			resp, err = stsClient.GetSessionToken(context.Background(), &sts.GetSessionTokenInput{
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

		// 取得したセッショントークンをAWS Configに保存
		exec.Command("aws", "configure", "set", "aws_access_key_id", *resp.Credentials.AccessKeyId, "--profile", outProfile).Output()
		exec.Command("aws", "configure", "set", "aws_secret_access_key", *resp.Credentials.SecretAccessKey, "--profile", outProfile).Output()
		exec.Command("aws", "configure", "set", "aws_session_token", *resp.Credentials.SessionToken, "--profile", outProfile).Output()
		exec.Command("aws", "configure", "set", "region", "ap-northeast-1", "--profile", outProfile).Output()
		exec.Command("aws", "configure", "set", "output", "json", "--profile", outProfile).Output()
		exec.Command("aws", "configure", "set", "mfa_serial", serialNumber, "--profile", outProfile).Output()
	},
}

// Execute は、すべての子コマンドをルートコマンドに追加し、フラグを適切に設定します。
// これは main.main() によって呼び出されます。ルートコマンドに対して一度だけ実行すれば十分です。
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// ここでは、フラグと設定を定義します。
	// Cobra は永続フラグをサポートしており、ここで定義するとアプリケーション全体で有効になります。

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.aws-cli-mfa-login.yaml)")

	// また、Cobra はローカルフラグもサポートしており、これはこのアクションが直接呼び出されたときにのみ有効になります。
	rootCmd.Flags().StringVarP(&profile, "profile", "p", "default", "AWS profile name")
	rootCmd.Flags().StringVarP(&serialNumber, "serial-number", "s", "", `MFA identifier. If not specified, the value of mfa_serial in AWS configure specified in the --out-profile argument is used.
	Ex: arn:aws:iam::123456789012:mfa/user
	`)
	rootCmd.Flags().StringVarP(&outProfile, "out-profile", "o", "default-sts", "AWS profile name to be used in STS")
	rootCmd.Flags().Int32VarP(&durationSeconds, "duration-seconds", "d", 3600, "The duration, in seconds, that the credentials should remain valid.")
}
