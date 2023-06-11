package openstack

import (
	"fmt"
	"os"
	"strings"

	"github.com/fjboy/magic-pocket/pkg/global/logging"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"

	"github.com/fjboy/ec-tools/pkg/guest"
	"github.com/fjboy/ec-tools/pkg/openstack/compute"
	"github.com/fjboy/ec-tools/pkg/openstack/identity"

	"github.com/fjboy/ec-tools/common"
)

func getAuthedClient() (compute.ComputeClientV2, error) {
	authClient, err := identity.GetV3ClientFromEnv()
	if err != nil {
		logging.Error("获取认证客户端失败, %s", err)
		return compute.ComputeClientV2{}, fmt.Errorf("获取计算客户端失败")
	}
	computeClient, err := compute.GetComputeClientV2(authClient)
	if err != nil {
		logging.Error("获取计算客户端失败, %s", err)
		return compute.ComputeClientV2{}, fmt.Errorf("获取计算客户端失败")
	}
	computeClient.UpdateVersion()
	return computeClient, nil
}

func PrintVmQosSetting(clientServer compute.Server, serverServer compute.Server) {
	tableWriter := table.NewWriter()
	rowConfigAutoMerge := table.RowConfig{AutoMerge: true}
	tableWriter.AppendHeader(table.Row{"Server", "Bandwidth(KBytes/sec)", "Bandwidth(KBytes/sec)", "PPS", "PPS"}, rowConfigAutoMerge)
	tableWriter.AppendHeader(table.Row{"", "入", "出", "入", "出"}, rowConfigAutoMerge)
	tableWriter.AppendRow(
		table.Row{
			clientServer.Id + "(Client)",
			clientServer.Flavor.ExtraSpecs["quota:vif_inbound_burst"],
			clientServer.Flavor.ExtraSpecs["quota:vif_outbound_burst"],
			clientServer.Flavor.ExtraSpecs["quota:vif_inbound_pps_burst"],
			clientServer.Flavor.ExtraSpecs["quota:vif_outbound_pps_burst"],
		})
	tableWriter.AppendRow(
		table.Row{
			serverServer.Id + "(Server)",
			serverServer.Flavor.ExtraSpecs["quota:vif_inbound_burst"],
			serverServer.Flavor.ExtraSpecs["quota:vif_outbound_burst"],
			serverServer.Flavor.ExtraSpecs["quota:vif_inbound_pps_burst"],
			serverServer.Flavor.ExtraSpecs["quota:vif_outbound_pps_burst"],
		})

	tableWriter.SetOutputMirror(os.Stdout)
	tableWriter.SetStyle(table.StyleLight)
	tableWriter.Style().Format.Header = text.FormatDefault
	tableWriter.Style().Options.SeparateRows = true
	tableWriter.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AlignHeader: text.AlignCenter, Align: text.AlignCenter},
		{Number: 2, AlignHeader: text.AlignCenter, Align: text.AlignRight},
		{Number: 3, AlignHeader: text.AlignCenter, Align: text.AlignRight},
		{Number: 4, AlignHeader: text.AlignCenter, Align: text.AlignRight},
	})
	logging.Info("虚拟机信息:")
	tableWriter.Render()
}

func TestNetQos(clientId string, serverId string) {
	err := common.LoadConf()
	if err != nil {
		logging.Error("加载配置文件失败, %s", err)
		os.Exit(1)
	}
	common.LogConf(common.CONF)
	computeClient, err := getAuthedClient()
	if err != nil {
		os.Exit(1)
	}
	if clientId == "" {
		os.Exit(1)
	}
	if serverId == "" {
		os.Exit(1)
	}

	logging.Info("查询客户端和服务端虚拟机信息")
	clientVm := computeClient.ServerShow(clientId)
	serverVm := computeClient.ServerShow(serverId)
	if clientVm.Id == "" {
		logging.Error("虚拟机 %s 不存在", clientId)
		return
	}
	if serverVm.Id == "" {
		logging.Error("虚拟机 %s 不存在", serverId)
		return
	}
	if strings.ToUpper(serverVm.Status) != "ACTIVE" {
		logging.Error("期望虚拟机 %s 状态是 ACTIVE, 实际是 %s", serverVm.Id, serverVm.Status)
		return
	}

	if strings.ToUpper(clientVm.Status) != "ACTIVE" {
		logging.Error("期望虚拟机 %s 状态是 ACTIVE, 实际是 %s", clientVm.Id, clientVm.Status)
		return
	}
	if strings.ToUpper(serverVm.Status) != "ACTIVE" {
		logging.Error("期望虚拟机 %s 状态是 ACTIVE, 实际是 %s", serverVm.Id, serverVm.Status)
		return
	}
	PrintVmQosSetting(clientVm, serverVm)

	clientConn := guest.GuestConnection{Connection: clientVm.Host, Domain: clientVm.Id}
	serverConn := guest.GuestConnection{Connection: serverVm.Host, Domain: serverVm.Id}

	logging.Info("开始通过 QGA 测试")
	guest.TestNetQos(clientConn, serverConn)
}

func DelErrorServers() {
	computeClient, err := getAuthedClient()
	if err != nil {
		return
	}
	query := map[string]string{}
	query["status"] = "error"
	logging.Info("查询虚拟机")
	servers := computeClient.ServerList(query)
	if len(servers) == 0 {
		logging.Warning("无状态为ERROR的虚拟机")
		return
	}
	logging.Info("开始删除虚拟机")
	for _, server := range servers {
		logging.Info("删除虚拟机 %s(%s)", server.Id, server.Name)
		computeClient.ServerDelete(server.Id)
	}
}
