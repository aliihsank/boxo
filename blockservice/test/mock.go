package bstest

import (
	testinstance "github.com/aliihsank/boxo/bitswap/testinstance"
	tn "github.com/aliihsank/boxo/bitswap/testnet"
	"github.com/aliihsank/boxo/blockservice"
	mockrouting "github.com/aliihsank/boxo/routing/mock"
	delay "github.com/ipfs/go-ipfs-delay"
)

// Mocks returns |n| connected mock Blockservices
func Mocks(n int, opts ...blockservice.Option) []blockservice.BlockService {
	net := tn.VirtualNetwork(mockrouting.NewServer(), delay.Fixed(0))
	sg := testinstance.NewTestInstanceGenerator(net, nil, nil)

	instances := sg.Instances(n)

	var servs []blockservice.BlockService
	for _, i := range instances {
		servs = append(servs, blockservice.New(i.Blockstore(), i.Exchange, opts...))
	}
	return servs
}
