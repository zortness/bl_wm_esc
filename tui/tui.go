package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bl_wm_esc/client"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Active Tabs
const (
	TabConnecting = iota
	TabSettings
	TabTelemetry
)

// Styling definitions with rich Lipgloss aesthetics
var (
	accentColor = lipgloss.Color("#7D56F4")
	greenColor  = lipgloss.Color("#04B575")
	redColor    = lipgloss.Color("#FF4A4A")
	grayColor   = lipgloss.Color("#8E8E8E")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(accentColor).
			Padding(0, 2).
			MarginTop(1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor).
			Padding(1, 2).
			Width(60)

	tabStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(grayColor).
			Padding(0, 1)

	activeTabStyle = tabStyle.Copy().
			Bold(true).
			Foreground(accentColor).
			BorderForeground(accentColor)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(accentColor).
				Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Width(22)

	valueStyle = lipgloss.NewStyle().
			Foreground(greenColor).
			Bold(true).
			Width(25)

	helpStyle = lipgloss.NewStyle().
			Foreground(grayColor).
			Italic(true)

	successStyle = lipgloss.NewStyle().
			Foreground(greenColor).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(redColor).
			Bold(true)
)

type connectMsg struct {
	err error
}

type readSettingsMsg struct {
	data []byte
	err  error
}

type writeSettingsMsg struct {
	err error
}

type telemetryMsg struct {
	telemetry *client.Telemetry
	err       error
}

type tickMsg time.Time

type TUIModel struct {
	client           *client.ESCClient
	activeTab        int
	selectedSetting  int
	settingsData     []byte // 50 bytes raw settings payload
	telemetryData    *client.Telemetry
	err              error
	statusMsg        string
	statusTimer      *time.Timer
	progress         progress.Model
	spinnerIndex     int
	lastPoll         time.Time
}

func NewModel(escClient *client.ESCClient) TUIModel {
	return TUIModel{
		client:    escClient,
		activeTab: TabConnecting,
		progress:  progress.New(progress.WithoutPercentage()),
	}
}

func (m TUIModel) Init() tea.Cmd {
	return tea.Batch(
		m.connectCmd(),
		m.spinnerTick(),
	)
}

func (m TUIModel) connectCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := m.client.Connect(ctx)
		return connectMsg{err: err}
	}
}

func (m TUIModel) readSettingsCmd() tea.Cmd {
	return func() tea.Msg {
		data, err := m.client.ReadSettings(m.client.ActiveProfile())
		return readSettingsMsg{data: data, err: err}
	}
}

func (m TUIModel) writeSettingsCmd() tea.Cmd {
	return func() tea.Msg {
		err := m.client.WriteSettings(m.client.ActiveProfile(), m.settingsData)
		return writeSettingsMsg{err: err}
	}
}

func (m TUIModel) telemetryCmd() tea.Cmd {
	return func() tea.Msg {
		t, err := m.client.QueryTelemetry()
		return telemetryMsg{telemetry: t, err: err}
	}
}

func (m TUIModel) spinnerTick() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.client.Close()
			return m, tea.Quit
		}

		if m.activeTab == TabConnecting {
			// Ignore other inputs during connection screen
			return m, nil
		}

		switch msg.String() {
		case "tab":
			// Toggle between tabs
			if m.activeTab == TabSettings {
				m.activeTab = TabTelemetry
				return m, m.telemetryCmd()
			} else if m.activeTab == TabTelemetry {
				m.activeTab = TabSettings
				return m, nil
			}

		case "up", "k":
			if m.activeTab == TabSettings {
				m.selectedSetting--
				if m.selectedSetting < 0 {
					m.selectedSetting = len(client.SettingOptions) - 1
				}
			}

		case "down", "j":
			if m.activeTab == TabSettings {
				m.selectedSetting++
				if m.selectedSetting >= len(client.SettingOptions) {
					m.selectedSetting = 0
				}
			}

		case "left", "h":
			if m.activeTab == TabSettings && len(m.settingsData) >= 50 {
				opt := client.SettingOptions[m.selectedSetting]
				realOffset := getRealOffset(opt.Offset)
				currVal := m.settingsData[realOffset]
				if currVal >= opt.Min+opt.Step {
					m.settingsData[realOffset] -= opt.Step
				} else {
					m.settingsData[realOffset] = opt.Min
				}
				m.statusMsg = ""
				m.err = nil
			}

		case "right", "l":
			if m.activeTab == TabSettings && len(m.settingsData) >= 50 {
				opt := client.SettingOptions[m.selectedSetting]
				realOffset := getRealOffset(opt.Offset)
				currVal := m.settingsData[realOffset]
				if currVal+opt.Step <= opt.Max {
					m.settingsData[realOffset] += opt.Step
				} else {
					m.settingsData[realOffset] = opt.Max
				}
				m.statusMsg = ""
				m.err = nil
			}

		case "ctrl+s":
			if m.activeTab == TabSettings && len(m.settingsData) >= 50 {
				m.statusMsg = "Saving settings to ESC..."
				m.err = nil
				return m, m.writeSettingsCmd()
			}
		}

	case connectMsg:
		if msg.err != nil {
			m.err = msg.err
			// Retry connection after 2 seconds
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return connectMsg{err: m.client.Connect(context.Background())}
			})
		}
		// Success connection
		m.err = nil
		m.activeTab = TabSettings
		m.statusMsg = "Connected! Loading settings..."
		return m, m.readSettingsCmd()

	case readSettingsMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("read settings failed: %v", msg.err)
			m.statusMsg = ""
			return m, nil
		}
		m.settingsData = msg.data
		m.statusMsg = "Settings loaded successfully."
		m.err = nil
		return m, nil

	case writeSettingsMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("save failed: %v", msg.err)
			m.statusMsg = ""
			return m, nil
		}
		m.statusMsg = "Settings saved to ESC successfully! Motor ready."
		m.err = nil
		return m, nil

	case telemetryMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("telemetry error: %v", msg.err)
			// Keep trying
		} else {
			m.telemetryData = msg.telemetry
			m.err = nil
		}
		
		// If still on telemetry tab, query again after 200ms
		if m.activeTab == TabTelemetry {
			return m, tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
				return m.telemetryCmd()()
			})
		}

	case tickMsg:
		m.spinnerIndex = (m.spinnerIndex + 1) % 4
		return m, m.spinnerTick()
	}

	return m, cmd
}

func (m TUIModel) View() string {
	var sb strings.Builder

	// Top Title Banner
	sb.WriteString(titleStyle.Render(" BL-WM ESC UTILITY ") + "\n")

	// Connection Information
	if m.client.IsConnected() {
		sb.WriteString(lipgloss.NewStyle().Foreground(grayColor).Render(
			fmt.Sprintf(" ESC: %s | Firmware: %s | Active Profile: %d", m.client.ModelName(), m.client.Firmware(), m.client.ActiveProfile())) + "\n\n")
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(redColor).Render(" Disconnected") + "\n\n")
	}

	// Render based on state
	switch m.activeTab {
	case TabConnecting:
		sb.WriteString(m.viewConnecting())
	case TabSettings:
		sb.WriteString(m.viewTabs())
		sb.WriteString(m.viewSettingsEditor())
	case TabTelemetry:
		sb.WriteString(m.viewTabs())
		sb.WriteString(m.viewTelemetry())
	}

	// Footer Status Area
	sb.WriteString("\n")
	if m.err != nil {
		sb.WriteString(errorStyle.Render(fmt.Sprintf(" Error: %v", m.err)) + "\n")
	} else if m.statusMsg != "" {
		sb.WriteString(successStyle.Render(fmt.Sprintf(" Status: %s", m.statusMsg)) + "\n")
	} else {
		sb.WriteString("\n")
	}

	// Help bar
	sb.WriteString("\n")
	if m.activeTab == TabConnecting {
		sb.WriteString(helpStyle.Render(" Ctrl+C: Quit") + "\n")
	} else if m.activeTab == TabSettings {
		sb.WriteString(helpStyle.Render(" Tab: Switch Tab | ↑/↓: Select | ←/→: Adjust | Ctrl+S: Save Settings | q: Quit") + "\n")
	} else {
		sb.WriteString(helpStyle.Render(" Tab: Switch Tab | q: Quit") + "\n")
	}

	return sb.String()
}

func (m TUIModel) viewConnecting() string {
	spinners := []string{"⠋", "⠙", "⠹", "⠸"}
	spinner := spinners[m.spinnerIndex]

	var sb strings.Builder
	sb.WriteString("╔════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                                                        ║\n")
	sb.WriteString("║               BL-WM ESC CONFIGURATION                  ║\n")
	sb.WriteString("║                                                        ║\n")
	sb.WriteString(fmt.Sprintf("║          %s Searching for ESC Wi-Fi...                  ║\n", spinner))
	sb.WriteString("║         [▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒]               ║\n")
	sb.WriteString("║                                                        ║\n")
	sb.WriteString("║      Connect your computer to the ESC's hotspot        ║\n")
	sb.WriteString("║              (default SSID: CAR X)                     ║\n")
	if m.err != nil {
		sb.WriteString("║                                                        ║\n")
		errStr := m.err.Error()
		if len(errStr) > 46 {
			errStr = errStr[:43] + "..."
		}
		padding := (46 - len(errStr)) / 2
		rightPadding := 46 - len(errStr) - padding
		sb.WriteString(fmt.Sprintf("║      %s%s%s      ║\n", strings.Repeat(" ", padding), errorStyle.Render(errStr), strings.Repeat(" ", rightPadding)))
	} else {
		sb.WriteString("║                                                        ║\n")
	}
	sb.WriteString("║                                                        ║\n")
	sb.WriteString("╚════════════════════════════════════════════════════════╝\n")
	
	return sb.String()
}

func (m TUIModel) viewTabs() string {
	settingsTab := " Settings Editor "
	telemetryTab := " Live Telemetry "

	if m.activeTab == TabSettings {
		return activeTabStyle.Render(settingsTab) + tabStyle.Render(telemetryTab) + "\n\n"
	}
	return tabStyle.Render(settingsTab) + activeTabStyle.Render(telemetryTab) + "\n\n"
}

func getRealOffset(offset int) int {
	if offset < 4 {
		return offset
	}
	if offset >= 4 && offset <= 24 {
		return offset + 1
	}
	return offset
}

func (m TUIModel) formatSettingValue(opt client.SettingOption, rawVal byte) string {
	// Cut Off Voltage special formatting
	if opt.Offset == client.PS_CUT_OFF_VOLT {
		if rawVal == 0 {
			return "Disable"
		}
		if rawVal == 1 {
			return "Auto"
		}
		if rawVal >= 2 {
			return fmt.Sprintf("%.1f V", 3.0+float32(rawVal-2)*0.1)
		}
	}

	// Current limiters special formatting
	if (opt.Offset == client.PS_START_CUR_LIMIT || opt.Offset == client.PS_CURRENT_LIMIT) && rawVal == 0 {
		return "Off"
	}

	// Mid Brake Amount special formatting
	if opt.Offset == client.PS_MID_BRAKE && rawVal == 0 {
		return "50% (Default)"
	}

	// Motor Pole Number special formatting (stored as raw value, display as rawVal * 2)
	if opt.Offset == client.PS_POLE_NUMBER {
		return fmt.Sprintf("%d%s", rawVal*2, opt.Suffix)
	}

	// M-Reverse Amount special formatting (stored as raw value, display as rawVal * 10)
	if opt.Offset == client.PS_MAX_REVERSE {
		return fmt.Sprintf("%d%s", rawVal*10, opt.Suffix)
	}

	// Turbo Delay / Slope special formatting (multiplier 0.05)
	if opt.Offset == client.PS_TURBO_DELAY || opt.Offset == client.PS_TURBO_INC || opt.Offset == client.PS_TURBO_DEC {
		return fmt.Sprintf("%.2f%s", float32(rawVal)*0.05, opt.Suffix)
	}

	// Labeled values formatting (support non-1 steps)
	if len(opt.Labels) > 0 {
		step := opt.Step
		if step == 0 {
			step = 1
		}
		idx := int((rawVal - opt.Min) / step)
		if idx >= 0 && idx < len(opt.Labels) {
			return opt.Labels[idx]
		}
		return fmt.Sprintf("%d (raw)", rawVal)
	}

	if opt.IsFloat {
		return fmt.Sprintf("%.1f%s", float32(rawVal)/10.0, opt.Suffix)
	}

	return fmt.Sprintf("%d%s", rawVal, opt.Suffix)
}

func (m TUIModel) viewSettingsEditor() string {
	if len(m.settingsData) < 50 {
		return "  [Loading Settings...]\n"
	}

	var sb strings.Builder
	for idx, opt := range client.SettingOptions {
		realOffset := getRealOffset(opt.Offset)
		rawVal := m.settingsData[realOffset]
		valStr := m.formatSettingValue(opt, rawVal)

		// Render active line selection
		if idx == m.selectedSetting {
			line := fmt.Sprintf(" > %-25s :  %s", opt.Name, valStr)
			sb.WriteString(selectedItemStyle.Render(line) + "\n")
		} else {
			sb.WriteString(fmt.Sprintf("   %-25s :  %s\n", labelStyle.Render(opt.Name), valueStyle.Render(valStr)))
		}
	}
	return sb.String()
}

func (m TUIModel) viewTelemetry() string {
	if m.telemetryData == nil {
		return "  [Waiting for Telemetry...]\n"
	}

	t := m.telemetryData
	var sb strings.Builder

	// Build two columns of telemetry dashboard
	col1 := []string{
		fmt.Sprintf(" Voltage   : %s", valueStyle.Render(fmt.Sprintf("%.2f V  (Min: %.2f V)", t.Voltage, t.VoltageMin))),
		fmt.Sprintf(" Current   : %s", valueStyle.Render(fmt.Sprintf("%.1f A  (Max: %.1f A)", t.Current, t.CurrentMax))),
		fmt.Sprintf(" ESC Temp  : %s", valueStyle.Render(fmt.Sprintf("%d °C  (Max: %d °C)", t.TempESC, t.TempESCMax))),
		fmt.Sprintf(" Motor Temp: %s", valueStyle.Render(fmt.Sprintf("%d °C  (Max: %d °C)", t.TempMotor, t.TempMotorMax))),
	}

	col2 := []string{
		fmt.Sprintf(" RPM       : %s", valueStyle.Render(fmt.Sprintf("%d rpm  (Max: %d rpm)", t.RPM, t.RPMMax))),
		fmt.Sprintf(" Throttle  : %s", valueStyle.Render(fmt.Sprintf("%.1f %%", t.Throttle))),
		fmt.Sprintf(" Run Time  : %s", valueStyle.Render(fmt.Sprintf("%.1f s", t.RunTime))),
	}

	sb.WriteString("╔══════════════════════ TELEMETRY MONITOR ══════════════════════╗\n")
	for i := 0; i < len(col1); i++ {
		line2 := ""
		if i < len(col2) {
			line2 = col2[i]
		}
		// Render clean table spacing
		sb.WriteString(fmt.Sprintf("║  %-45s %-32s║\n", col1[i], line2))
	}
	sb.WriteString("╚═══════════════════════════════════════════════════════════════╝\n")

	return sb.String()
}
