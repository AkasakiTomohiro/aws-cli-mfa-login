package internal

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// 引数で指定されたプロファイルから認証情報を取得
func LoadAWSProfile(ctx context.Context, profile string, outProfile string, updateSerialNumber bool) (aws.Config, string) {

	// AWS Configから指定されたプロファイルの認証情報を取得
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	if err != nil {
		// プロファイルが存在しない場合はエラー
		log.Fatalln(err)
	}

	// AWS ConfigからMFAのシリアル番号を取得
	var serialNumber = ""
	outCfg, err := config.LoadSharedConfigProfile(ctx, outProfile)
	if err == nil {
		serialNumber = outCfg.MFASerial
	}

	// AWS ConfigにMFAのシリアル番号が指定されていない場合はIAMユーザーの仮想MFAデバイスを取得する
	if serialNumber == "" || updateSerialNumber {
		serialNumber = getVirtualMfaDevice(ctx, cfg)

		// 仮想デバイスの選択をキャンセルした場合はエラー
		if serialNumber == ABORT_SELECT_SERIAL_NUMBER {
			log.Fatalln("Selection of the virtual MFA device has been interrupted.")
		}

		// 仮想デバイスが1つも登録されていない場合はエラー
		if serialNumber == NOT_FOUND_SERIAL_NUMBER {
			log.Fatalln("Virtual MFA device is not set up. Please log in to the AWS Management Console and configure a virtual MFA device.")
		}
	}

	return cfg, serialNumber
}
