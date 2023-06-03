package guest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/fjboy/magic-pocket/pkg/global/logging"
	"libvirt.org/go/libvirt"
)

type GuestExecArguments struct {
	CaptureOutput bool     `json:"capture-output"`
	Path          string   `json:"path"`
	Arg           []string `json:"arg"`
}
type GuestExecStatusArguments struct {
	Pid int `json:"pid"`
}

type QemuAgentCommand struct {
	Execute   string             `json:"execute"`
	Arguments GuestExecArguments `json:"arguments"`
}
type QACExecStatus struct {
	Execute   string                   `json:"execute"`
	Arguments GuestExecStatusArguments `json:"arguments"`
}
type QgaExecReturn struct {
	Pid int `json:"pid"`
}
type QgaExecStatusReturn struct {
	Exited  bool   `json:"exited"`
	OutData string `json:"out-data"`
	ErrData string `json:"err-data"`
}
type QgaExecResult struct {
	Return QgaExecReturn `json:"return"`
}
type QgaExecStatusResult struct {
	Return QgaExecStatusReturn `json:"return"`
}

func getGuestExecArguments(command string) GuestExecArguments {
	commandArgs := strings.Split(command, " ")
	return GuestExecArguments{
		CaptureOutput: true,
		Path:          commandArgs[0],
		Arg:           commandArgs[1:],
	}
}
func getGuestExecStatusArguments(pid int) GuestExecStatusArguments {
	return GuestExecStatusArguments{
		Pid: pid,
	}
}

type ExecResult struct {
	Pid     int
	OutData string
	ErrData string
}

func (guest *Guest) Exec(command string, wait bool) ExecResult {
	qemuAgentCommand := QemuAgentCommand{
		Execute:   "guest-exec",
		Arguments: getGuestExecArguments(command),
	}
	jsonData, _ := json.Marshal(qemuAgentCommand)
	result, err := guest.runQemuAgentCommand(jsonData)
	if err != nil {
		return ExecResult{}
	}
	var qgaExecResult QgaExecResult
	json.Unmarshal([]byte(result), &qgaExecResult)
	if !wait {
		return ExecResult{Pid: qgaExecResult.Return.Pid}
	}
	outData, errData := guest.getExecStatusOutput(qgaExecResult.Return.Pid)
	return ExecResult{
		Pid:     qgaExecResult.Return.Pid,
		OutData: outData,
		ErrData: errData,
	}
}

func (guest *Guest) runQemuAgentCommand(jsonData []byte) (string, error) {
	logging.Debug("QGA 命令: %s", fmt.Sprintf("%s", jsonData))
	result, err := guest.domain.QemuAgentCommand(
		fmt.Sprintf("%s", jsonData),
		libvirt.DOMAIN_QEMU_AGENT_COMMAND_MIN,
		0)
	if err != nil {
		logging.Error("执行失败: %s", err)
		return "", err
	}
	logging.Debug("命令执行结果: %s", result)
	return result, nil
}

// guest-exec-status
func (guest *Guest) getExecStatusOutput(pid int) (string, string) {
	qemuAgentCommand := QACExecStatus{
		Execute:   "guest-exec-status",
		Arguments: getGuestExecStatusArguments(pid),
	}
	jsonData, _ := json.Marshal(qemuAgentCommand)
	var qgaExecResult QgaExecStatusResult
	startTime := time.Now()
	for {
		result, _ := guest.runQemuAgentCommand(jsonData)
		json.Unmarshal([]byte(result), &qgaExecResult)
		if qgaExecResult.Return.Exited {
			break
		}
		if guest.QGATimeout > 0 &&
			time.Since(startTime).Seconds() >= float64(time.Second)*float64(guest.QGATimeout) {
			break
		}
		time.Sleep(time.Second * 2)
	}
	outDecode, _ := base64.StdEncoding.DecodeString(qgaExecResult.Return.OutData)
	errDecode, _ := base64.StdEncoding.DecodeString(qgaExecResult.Return.ErrData)
	return string(outDecode), string(errDecode)
}

type AddrInfo struct {
	Famaliy   string `json:"family"`
	Local     string `json:"local"`
	Label     string `json:"label"`
	Broadcast string `json:"broadcast"`
}

type IPAddress struct {
	IFIndex  int        `json:"ifindex"`
	AddrInfo []AddrInfo `json:"addr_info"`
}

func (guest *Guest) GetIpaddrs() []string {
	execResult := guest.Exec("ip a", true)
	reg := regexp.MustCompile("inet [0-9.]+")
	matchedIPAddresses := reg.FindAllString(execResult.OutData, -1)
	ipAddresses := []string{}
	for i := 0; i < len(matchedIPAddresses); i++ {
		if strings.Contains(matchedIPAddresses[i], "127.0.0.1") {
			continue
		}
		ipAddresses = append(ipAddresses, strings.Split(matchedIPAddresses[i], " ")[1])
	}
	return ipAddresses
}

// Return pid
func (guest *Guest) RunIperfServer(serverIp string, logfile string) ExecResult {
	cmd := fmt.Sprintf("iperf3 -s --bind %s --logfile %s", serverIp, logfile)
	return guest.Exec(cmd, false)
}

// Return pid
func (guest *Guest) RunIperfClient(clientIp string, serverIp string, logfile string) ExecResult {
	cmd := fmt.Sprintf("iperf3 -c %s --bind %s --logfile %s", serverIp, clientIp, logfile)
	return guest.Exec(cmd, false)
}
