package internal

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
)

// ユーザーからMFAの認証コードを入力してもらい、認証情報を取得
func GetSessionToken(ctx context.Context, cfg aws.Config, serialNumber string, durationSeconds int32) *types.Credentials {

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
