package client

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// ESC Client constants
const (
	ESC_IP   = "192.168.4.1"
	ESC_PORT = 7788
)

// Settings offset indices
const (
	PS_BATT_TYPE        = 0
	PS_CUT_OFF_VOLT     = 1
	PS_POWER_CURVE      = 2
	PS_ADVANCE_TIME     = 3
	PS_ACCELERATION     = 4
	PS_START_POWER      = 5
	PS_START_CUR_LIMIT  = 6
	PS_CURRENT_LIMIT    = 7
	PS_REVERSE_FUNCTION = 8
	PS_REVERSE_DELAY    = 9
	PS_NEUTRAL_WIDTH    = 10
	PS_DIRECTION        = 11
	PS_SPEED_BRAKE      = 12
	PS_ABS_FUNCTION     = 13
	PS_AUTO_BRAKE       = 14
	PS_MIN_BRAKE        = 15
	PS_MAX_BRAKE        = 16
	PS_POLE_NUMBER      = 22
	PS_GEAR_RATIO       = 23
	PS_TIRE_DIA         = 24
	PS_BEC_VOLT         = 35
	PS_CUT_TEMP1        = 36
	PS_CUT_TEMP2        = 37
	PS_MAX_REVERSE      = 38
	PS_MOTO_WIRING      = 39
	PS_MID_BRAKE        = 40
	PS_MID_BRAKE_LOC    = 41
	PS_BRAKE_SOFT       = 42
	PS_FREQ_BRAKE       = 43
	PS_FREQ_PWM         = 44
	PS_FREQ_DRAG        = 45
	PS_TURBO_DELAY      = 46
	PS_TURBO_INC        = 47
	PS_TURBO_DEC        = 48
)

type SettingOption struct {
	Name    string
	Offset  int
	Min     byte
	Max     byte
	Step    byte
	Labels  []string
	IsFloat bool
	Suffix  string
}

var SettingOptions = []SettingOption{
	// General Tab
	{Name: "Battery Type", Offset: PS_BATT_TYPE, Min: 0, Max: 2, Step: 1, Labels: []string{"NiMH/NiCd", "LiPo", "LiFe"}},
	{Name: "Cut Off Voltage", Offset: PS_CUT_OFF_VOLT, Min: 0, Max: 47, Step: 1},
	{Name: "Motor Direction", Offset: PS_DIRECTION, Min: 0, Max: 1, Step: 1, Labels: []string{"Normal", "Reverse"}},
	{Name: "Motor Pole Number", Offset: PS_POLE_NUMBER, Min: 1, Max: 10, Step: 1, Suffix: " Pole"},
	{Name: "Gear Ratio", Offset: PS_GEAR_RATIO, Min: 20, Max: 150, Step: 1, IsFloat: true, Suffix: " : 1"},
	{Name: "Tire Diameter", Offset: PS_TIRE_DIA, Min: 40, Max: 200, Step: 1, Suffix: " mm"},
	{Name: "BEC Voltage", Offset: PS_BEC_VOLT, Min: 0, Max: 1, Step: 1, Labels: []string{"6.0 V", "7.4 V"}},
	{Name: "Cut Off Temp", Offset: PS_CUT_TEMP1, Min: 0, Max: 8, Step: 1, Labels: []string{"100°C / 212°F", "105°C / 221°F", "110°C / 230°F", "115°C / 239°F", "120°C / 248°F", "125°C / 257°F", "130°C / 266°F", "135°C / 275°F", "Disabled"}},
	{Name: "Cut Off Motor Temp", Offset: PS_CUT_TEMP2, Min: 0, Max: 8, Step: 1, Labels: []string{"100°C / 212°F", "105°C / 221°F", "110°C / 230°F", "115°C / 239°F", "120°C / 248°F", "125°C / 257°F", "130°C / 266°F", "135°C / 275°F", "Disabled"}},
	{Name: "Motor Wiring", Offset: PS_MOTO_WIRING, Min: 50, Max: 80, Step: 30, Labels: []string{"A-B-C", "C-B-A"}},

	// Throttle Tab
	{Name: "Power Curve", Offset: PS_POWER_CURVE, Min: 0, Max: 10, Step: 1},
	{Name: "Acceleration", Offset: PS_ACCELERATION, Min: 0, Max: 10, Step: 1},
	{Name: "Start Power", Offset: PS_START_POWER, Min: 0, Max: 100, Step: 1, Suffix: " %"},
	{Name: "Start Current Limit", Offset: PS_START_CUR_LIMIT, Min: 0, Max: 100, Step: 1, Suffix: " %"},
	{Name: "Current Limit", Offset: PS_CURRENT_LIMIT, Min: 0, Max: 100, Step: 1, Suffix: " %"},
	{Name: "Neutral Width", Offset: PS_NEUTRAL_WIDTH, Min: 0, Max: 2, Step: 1, Labels: []string{"Narrow", "Normal", "Wide"}},

	// Reverse Tab
	{Name: "Reverse Function", Offset: PS_REVERSE_FUNCTION, Min: 0, Max: 3, Step: 1, Labels: []string{"One Way", "Two Way", "Two Way 2", "Two Way 3"}},
	{Name: "Reverse Delay", Offset: PS_REVERSE_DELAY, Min: 0, Max: 6, Step: 1, Labels: []string{"Off", "0.2s", "0.5s", "0.8s", "1.3s", "1.8s", "2.5s"}},
	{Name: "M-Reverse Amount", Offset: PS_MAX_REVERSE, Min: 2, Max: 10, Step: 1, Suffix: " %"},

	// Brake Tab
	{Name: "Brake Response", Offset: PS_SPEED_BRAKE, Min: 0, Max: 100, Step: 1, Suffix: " %"},
	{Name: "ABS Brake", Offset: PS_ABS_FUNCTION, Min: 0, Max: 5, Step: 1, Labels: []string{"Off", "Weakest", "Weak", "Normal", "Strong", "Strongest"}},
	{Name: "Drag Brake", Offset: PS_AUTO_BRAKE, Min: 0, Max: 100, Step: 1, Suffix: " %"},
	{Name: "Min Brake Amount", Offset: PS_MIN_BRAKE, Min: 0, Max: 100, Step: 1, Suffix: " %"},
	{Name: "Max Brake Amount", Offset: PS_MAX_BRAKE, Min: 0, Max: 100, Step: 1, Suffix: " %"},
	{Name: "Mid Brake Amount", Offset: PS_MID_BRAKE, Min: 0, Max: 100, Step: 1, Suffix: " %"},
	{Name: "Mid Brake Location", Offset: PS_MID_BRAKE_LOC, Min: 0, Max: 100, Step: 1, Suffix: " %"},
	{Name: "Soft Brake", Offset: PS_BRAKE_SOFT, Min: 1, Max: 50, Step: 49, Labels: []string{"Hard Brake", "Soft Brake"}},

	// Frequency Tab
	{Name: "Brake Frequency", Offset: PS_FREQ_BRAKE, Min: 0, Max: 5, Step: 1, Labels: []string{"1 kHz", "2 kHz", "5 kHz", "8 kHz", "16 kHz", "32 kHz"}},
	{Name: "Motor Frequency", Offset: PS_FREQ_PWM, Min: 0, Max: 5, Step: 1, Labels: []string{"1 kHz", "2 kHz", "5 kHz", "8 kHz", "16 kHz", "32 kHz"}},
	{Name: "Drag Frequency", Offset: PS_FREQ_DRAG, Min: 0, Max: 5, Step: 1, Labels: []string{"1 kHz", "2 kHz", "5 kHz", "8 kHz", "16 kHz", "32 kHz"}},

	// Boost/Turbo Tab
	{Name: "Turbo Delay", Offset: PS_TURBO_DELAY, Min: 0, Max: 20, Step: 1, IsFloat: true, Suffix: " s"},
	{Name: "Turbo +Slope", Offset: PS_TURBO_INC, Min: 0, Max: 20, Step: 1, IsFloat: true, Suffix: " s"},
	{Name: "Turbo -Slope", Offset: PS_TURBO_DEC, Min: 0, Max: 20, Step: 1, IsFloat: true, Suffix: " s"},
}

// Telemetry struct representing parsed live values
type Telemetry struct {
	Voltage      float32
	VoltageMin   float32
	Current      float32
	CurrentMax   float32
	TempESC      int
	TempESCMax   int
	TempMotor    int
	TempMotorMax int
	RPM          int
	RPMMax       int
	Throttle     float32
	RunTime      float32
}

type ESCClient struct {
	conn          net.Conn
	mu            sync.Mutex
	connected     bool
	modelName     string
	firmware      string
	activeProfile int
}

func NewESCClient() *ESCClient {
	return &ESCClient{}
}

// ComputeCRC calculates CRC-16-CCITT (Poly: 0x1021, Init: 0x0000)
func ComputeCRC(data []byte) uint16 {
	var crc uint16 = 0
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if (crc & 0x8000) != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc = crc << 1
			}
		}
	}
	return crc
}

// SendAndReceive sends a command frame and reads the response frame
func (c *ESCClient) SendAndReceive(payload []byte, timeout time.Duration) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Set deadline
	c.conn.SetDeadline(time.Now().Add(timeout))

	// Construct request frame: Magic (2 bytes) + Length (2 bytes) + Payload + CRC (2 bytes)
	length := len(payload)
	frame := make([]byte, 6+length)
	frame[0] = 0x00
	frame[1] = 0x00 // Request magic
	binary.LittleEndian.PutUint16(frame[2:4], uint16(length))
	copy(frame[4:4+length], payload)

	// Calculate and write CRC
	crc := ComputeCRC(frame[0 : 4+length])
	binary.LittleEndian.PutUint16(frame[4+length:6+length], crc)

	// Write to connection
	_, err := c.conn.Write(frame)
	if err != nil {
		c.connected = false
		return nil, fmt.Errorf("write error: %v", err)
	}

	// Read response header: Magic (2 bytes) + Length (2 bytes)
	respHeader := make([]byte, 4)
	_, err = io.ReadFull(c.conn, respHeader)
	if err != nil {
		c.connected = false
		return nil, fmt.Errorf("read header error: %v", err)
	}

	if respHeader[0] != 0x00 || respHeader[1] != 0x01 {
		return nil, fmt.Errorf("invalid response magic: %x %x", respHeader[0], respHeader[1])
	}

	respLen := binary.LittleEndian.Uint16(respHeader[2:4])
	
	// Read payload + CRC
	respBody := make([]byte, respLen+2)
	_, err = io.ReadFull(c.conn, respBody)
	if err != nil {
		c.connected = false
		return nil, fmt.Errorf("read body error: %v", err)
	}

	// Verify CRC
	fullResp := append(respHeader, respBody...)
	calcCRC := ComputeCRC(fullResp[0 : 4+respLen])
	expectedCRC := binary.LittleEndian.Uint16(respBody[respLen : respLen+2])

	if calcCRC != expectedCRC {
		return nil, fmt.Errorf("CRC mismatch: calculated %04x, expected %04x", calcCRC, expectedCRC)
	}

	return respBody[0:respLen], nil
}

// Connect attempts to connect to the ESC TCP server
func (c *ESCClient) Connect(ctx context.Context) error {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", ESC_IP, ESC_PORT))
	if err != nil {
		return err
	}

	c.conn = conn
	c.connected = true

	// Query device info to check validity
	infoPayload, err := c.QueryInfo()
	if err != nil {
		conn.Close()
		c.connected = false
		return fmt.Errorf("failed to query info: %v", err)
	}

	// Parse model and firmware
	c.parseDeviceInfo(infoPayload)

	// Query active profile
	activeProfile, err := c.QueryActiveProfile()
	if err != nil {
		c.activeProfile = 0 // default fallback
	} else {
		c.activeProfile = activeProfile
	}

	return nil
}

func (c *ESCClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connected = false
}

func (c *ESCClient) IsConnected() bool {
	return c.connected
}

func (c *ESCClient) ModelName() string {
	return c.modelName
}

func (c *ESCClient) Firmware() string {
	return c.firmware
}

func (c *ESCClient) parseDeviceInfo(payload []byte) {
	// Payload looks like: e2 bf b7 00 68 <string>
	if len(payload) < 5 {
		c.modelName = "Unknown ESC"
		c.firmware = "Unknown"
		return
	}

	strBytes := payload[5:]
	// Find double spaces or trailing noise to clean up
	fullStr := string(strBytes)
	parts := strings.Split(fullStr, "  ")
	
	cleanedParts := []string{}
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			cleanedParts = append(cleanedParts, trimmed)
		}
	}

	if len(cleanedParts) >= 2 {
		c.modelName = cleanedParts[0]
		c.firmware = cleanedParts[1]
	} else if len(cleanedParts) == 1 {
		c.modelName = cleanedParts[0]
		c.firmware = "N/A"
	} else {
		c.modelName = "GM Genius Pro"
		c.firmware = "N/A"
	}
}

// QueryInfo queries device info (Command 0x04)
func (c *ESCClient) QueryInfo() ([]byte, error) {
	return c.SendAndReceive([]byte{0x04}, 2*time.Second)
}

// ReadSettings reads active settings for a profile (Command 0x02 0x01 [profile] 0x00)
func (c *ESCClient) ReadSettings(profile int) ([]byte, error) {
	req := []byte{0x02, 0x01, byte(profile), 0x00}
	return c.SendAndReceive(req, 2*time.Second)
}

// ReadTimingSettings reads timing settings for a profile (Command 0x02 0x02 [profile] 0x00)
func (c *ESCClient) ReadTimingSettings(profile int) ([]byte, error) {
	req := []byte{0x02, 0x02, byte(profile), 0x00}
	return c.SendAndReceive(req, 2*time.Second)
}

// WriteTimingSettings writes timing settings back to the ESC (Command 0x03 0x02 [timingData...])
func (c *ESCClient) WriteTimingSettings(profile int, timingData []byte) error {
	req := make([]byte, 2+len(timingData))
	req[0] = 0x03
	req[1] = 0x02
	copy(req[2:], timingData)

	resp, err := c.SendAndReceive(req, 3*time.Second)
	if err != nil {
		return fmt.Errorf("write timing command failed: %v", err)
	}
	if len(resp) == 0 {
		return fmt.Errorf("empty timing write confirmation response")
	}
	return nil
}

// WriteSettings writes settings data back to the ESC, writing both common and timing settings
// to ensure the write transaction is complete, then sends the commit command.
func (c *ESCClient) WriteSettings(profile int, data []byte) error {
	// 1. Read current timing settings to include them in the transaction
	timingData, err := c.ReadTimingSettings(profile)
	if err != nil {
		return fmt.Errorf("failed to read timing settings for write transaction: %v", err)
	}

	// 2. Write common settings: 0x03, 0x01, then data (starts with ModelType)
	commonReq := make([]byte, 2+len(data))
	commonReq[0] = 0x03
	commonReq[1] = 0x01
	copy(commonReq[2:], data)

	resp, err := c.SendAndReceive(commonReq, 3*time.Second)
	if err != nil {
		return fmt.Errorf("write common settings failed: %v", err)
	}
	if len(resp) == 0 {
		return fmt.Errorf("empty write confirmation response")
	}

	// Wait 100ms
	time.Sleep(100 * time.Millisecond)

	// 3. Write timing settings: 0x03, 0x02, then timingData
	err = c.WriteTimingSettings(profile, timingData)
	if err != nil {
		return fmt.Errorf("write timing settings failed: %v", err)
	}

	// Wait 200ms before sending the commit command to give the ESC time to process the settings
	time.Sleep(200 * time.Millisecond)

	// 4. Send commit settings command: 0x00
	commitReq := []byte{0x00}
	commitResp, err := c.SendAndReceive(commitReq, 3*time.Second)
	if err != nil {
		// Since the ESC reboots immediately and closes the TCP connection, 
		// we treat read/write connection errors on the commit command as expected success behavior.
		if strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "reset") || strings.Contains(err.Error(), "closed") {
			return nil
		}
		return fmt.Errorf("commit command failed: %v", err)
	}

	if len(commitResp) == 0 {
		return fmt.Errorf("empty commit confirmation response")
	}

	return nil
}

// QueryTelemetry fetches live telemetry values (Command 0x18)
func (c *ESCClient) QueryTelemetry() (*Telemetry, error) {
	resp, err := c.SendAndReceive([]byte{0x18}, 1*time.Second)
	if err != nil {
		return nil, err
	}

	// Payload must be at least 50 bytes (mTime ends at index 49)
	if len(resp) < 50 {
		return nil, fmt.Errorf("telemetry packet too short: %d bytes", len(resp))
	}

	// Telemetry mapping based on verified dex disassembly of com.graupner.speedctrlm.model.EscData.InitData
	// Using offset = 0 (LITTLE_ENDIAN):
	// - mVolt: offset + 9 (2 bytes, Short, /10.0 for V)
	// - mVoltMin: offset + 11 (2 bytes, Short, /10.0 for V)
	// - mCapacity: offset + 13 (2 bytes, Short, mAh)
	// - mTemp (ESC Temp): offset + 15 (1 byte, °C) - Needs subtracting 20
	// - mTempMax: offset + 16 (1 byte, °C) - Needs subtracting 20
	// - mCurrent: offset + 17 (2 bytes, Short, /10.0 for A)
	// - mCurrentMax: offset + 19 (2 bytes, Short, /10.0 for A)
	// - mRpm: offset + 21 (2 bytes, Short, *10 for rpm)
	// - mRpmMax: offset + 23 (2 bytes, Short, *10 for rpm)
	// - mTempMotor: offset + 25 (1 byte, °C) - Needs subtracting 20
	// - mTempMotorMax: offset + 26 (1 byte, °C) - Needs subtracting 20
	// - mThrottle: offset + 32 (2 bytes, Short, % relative to 1023)
	// - mTime: offset + 46 (4 bytes, Int, tenths of a second)
	
	voltRaw := binary.LittleEndian.Uint16(resp[9:11])
	voltMinRaw := binary.LittleEndian.Uint16(resp[11:13])
	
	// Subtract 20 from raw temperature values as verified by dex assembly (add-int/lit8 v0, v0, #-20)
	tempESC := int(int8(resp[15])) - 20
	tempESCMax := int(int8(resp[16])) - 20
	
	currentRaw := binary.LittleEndian.Uint16(resp[17:19])
	currentMaxRaw := binary.LittleEndian.Uint16(resp[19:21])
	
	rpmRaw := binary.LittleEndian.Uint16(resp[21:23])
	rpmMaxRaw := binary.LittleEndian.Uint16(resp[23:25])
	
	tempMotor := int(int8(resp[25])) - 20
	tempMotorMax := int(int8(resp[26])) - 20
	
	// Scale throttle relative to 1023, and invert the sign (so forward throttle is positive)
	rawThrottle := int16(binary.LittleEndian.Uint16(resp[32:34]))
	throttle := -(float32(rawThrottle) / 1023.0 * 100.0)
	
	// Time is stored in tenths of a second (100ms units)
	runTimeRaw := binary.LittleEndian.Uint32(resp[46:50])
	runTime := float32(runTimeRaw) / 10.0

	var voltageMin float32
	if voltMinRaw == 31744 || voltMinRaw == 0x7c00 {
		voltageMin = 0.0 // Default/unset initialized state (infinity in FP16)
	} else {
		voltageMin = float32(voltMinRaw) / 10.0
	}

	return &Telemetry{
		Voltage:      float32(voltRaw) / 10.0,
		VoltageMin:   voltageMin,
		Current:      float32(currentRaw) / 10.0,
		CurrentMax:   float32(currentMaxRaw) / 10.0,
		TempESC:      tempESC,
		TempESCMax:   tempESCMax,
		TempMotor:    tempMotor,
		TempMotorMax: tempMotorMax,
		RPM:          int(rpmRaw) * 10,
		RPMMax:       int(rpmMaxRaw) * 10,
		Throttle:     throttle,
		RunTime:      runTime,
	}, nil
}

func (c *ESCClient) ActiveProfile() int {
	return c.activeProfile
}

// QueryActiveProfile queries the active profile index from the ESC (Command 0x23)
func (c *ESCClient) QueryActiveProfile() (int, error) {
	resp, err := c.SendAndReceive([]byte{0x23}, 2*time.Second)
	if err != nil {
		return 0, err
	}
	if len(resp) == 0 {
		return 0, fmt.Errorf("empty active profile response")
	}
	return int(resp[0]), nil
}
