package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/BytemanD/easygo/pkg/global/logging"
	"github.com/BytemanD/ec-tools/pkg/guest"
)

var (
	connection string
	uuid       bool
)
var QGACommand = &cobra.Command{
	Use:   "qga-exec <domain> <command>",
	Short: "QGA 命令执行工具",
	Long:  "执行 Libvirt QGA(qemu-guest-agent) 命令",
	Args:  cobra.ExactValidArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		domainName := args[0]
		command := args[1]
		domainGuest := guest.Guest{
			Connection: connection,
			Domain:     domainName,
			ByUUID:     uuid,
		}
		err := domainGuest.Connect()
		if err != nil {
			logging.Error("连接domain失败 %s", err)
			return
		}
		execResult := domainGuest.Exec(command, true)
		if execResult.OutData != "" {
			fmt.Println(execResult.OutData)
		}
		if execResult.ErrData != "" {
			fmt.Println(execResult.ErrData)
		}
	},
}

func init() {
	QGACommand.Flags().StringVarP(&connection, "connection", "c", "localhost", "连接地址")
	QGACommand.Flags().BoolVarP(&uuid, "uuid", "u", false, "通过 UUID 查找")
}
