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
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var profile string
var outProfile string
var serialNumber string
var selectSerialNumber string
var durationSeconds int32
var updateSerialNumber bool

const ABORT_SELECT_SERIAL_NUMBER = "ABORT_SELECT_SERIAL_NUMBER"
const NOT_FOUND_SERIAL_NUMBER = "NOT_FOUND_SERIAL_NUMBER"

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type item struct {
	deviceName string
	arn        string
}

func (i item) Title() string       { return i.deviceName }
func (i item) Description() string { return i.arn }
func (i item) FilterValue() string { return i.deviceName }

type model struct {
	list list.Model
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		{
			if msg.String() == "ctrl+c" {
				selectSerialNumber = ABORT_SELECT_SERIAL_NUMBER
				return m, tea.Quit
			}
			if msg.String() == "enter" {
				i, ok := m.list.SelectedItem().(item)
				if ok {
					selectSerialNumber = i.arn
					return m, tea.Quit
				}
			}
		}
	case tea.WindowSizeMsg:
		{
			h, v := docStyle.GetFrameSize()
			m.list.SetSize(msg.Width-h, msg.Height-v)
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return docStyle.Render(m.list.View())
}

// IAMユーザーの仮想MFAデバイス一覧を取得して、シリアル番号を１つ選択する
func getVirtualMfaDevice(ctx context.Context, cfg aws.Config) string {
	iamClient := iam.NewFromConfig(cfg)
	resp, err := iamClient.ListVirtualMFADevices(ctx, &iam.ListVirtualMFADevicesInput{})
	if err != nil {
		log.Fatalln(err)
	}
	if len(resp.VirtualMFADevices) == 0 {
		// 仮想MFAデバイスが1つも登録されていない場合
		return NOT_FOUND_SERIAL_NUMBER
	}
	if len(resp.VirtualMFADevices) == 1 {
		return *resp.VirtualMFADevices[0].SerialNumber
	}

	// 仮想MFAデバイスが複数ある場合は選択肢を表示し選択してもらう
	var devices []list.Item
	for _, device := range resp.VirtualMFADevices {
		deviceName := strings.Join(strings.Split((*device.SerialNumber), "/")[1:], "/")
		devices = append(devices, item{
			deviceName: deviceName,
			arn:        *device.SerialNumber,
		})
	}
	l := list.New(devices, list.NewDefaultDelegate(), 0, 0)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()
	l.InfiniteScrolling = true
	m := model{list: l}
	m.list.Title = "Select MFA Device"

	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	return selectSerialNumber
}

// 引数で指定されたプロファイルから認証情報を取得
func loadAWSProfile(ctx context.Context) aws.Config {

	// AWS Configから指定されたプロファイルの認証情報を取得
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	if err != nil {
		// プロファイルが存在しない場合はエラー
		log.Fatalln(err)
	}

	// AWS ConfigからMFAのシリアル番号を取得
	outCfg, err := config.LoadSharedConfigProfile(ctx, outProfile)
	if err != nil {
		log.Fatalln(err)
	}
	serialNumber = outCfg.MFASerial

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
	rootCmd.Flags().StringVarP(&outProfile, "out-profile", "o", "default-sts", "AWS profile name to be used in STS")
	rootCmd.Flags().Int32VarP(&durationSeconds, "duration-seconds", "d", 3600, "The duration, in seconds, that the credentials should remain valid.")
	rootCmd.Flags().BoolVarP(&updateSerialNumber, "update-serial-number", "u", false, "Update the MFA serial number")
}
