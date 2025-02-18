package internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var selectSerialNumber string

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
