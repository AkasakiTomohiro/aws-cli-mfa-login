package internal

import (
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/service/sts/types"
)

// 認証情報をAWS Configに保存
func SetSessionToken(outProfile string, serialNumber string, credentials *types.Credentials) {
	exec.Command("aws", "configure", "set", "aws_access_key_id", *credentials.AccessKeyId, "--profile", outProfile).Output()
	exec.Command("aws", "configure", "set", "aws_secret_access_key", *credentials.SecretAccessKey, "--profile", outProfile).Output()
	exec.Command("aws", "configure", "set", "aws_session_token", *credentials.SessionToken, "--profile", outProfile).Output()
	exec.Command("aws", "configure", "set", "region", "ap-northeast-1", "--profile", outProfile).Output()
	exec.Command("aws", "configure", "set", "output", "json", "--profile", outProfile).Output()
	exec.Command("aws", "configure", "set", "mfa_serial", serialNumber, "--profile", outProfile).Output()
}
