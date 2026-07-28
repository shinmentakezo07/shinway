package pluginhost

import (
	"context"

	"github.com/shinmentakezo07/shinway/v7/sdk/pluginabi"
)

const pluginHostABIVersion = pluginabi.ABIVersion

type pluginClient interface {
	Call(ctx context.Context, method string, request []byte) ([]byte, error)
	Shutdown()
}

type pluginLoader interface {
	Open(file pluginFile, host *Host) (pluginClient, error)
}
