package migrate

import (
	"encoding/json"
	"fmt"
	"io"
)

type Config struct {
	ImportPaths map[string]string
	Modules     []string
}

var DefaultConfig = Config{
	ImportPaths: map[string]string{
		"github.com/ipfs/go-bitswap":                     "github.com/aliihsank/boxo/bitswap",
		"github.com/ipfs/go-ipfs-files":                  "github.com/aliihsank/boxo/files",
		"github.com/ipfs/tar-utils":                      "github.com/aliihsank/boxo/tar",
		"github.com/ipfs/interface-go-ipfs-core":         "github.com/aliihsank/boxo/coreiface",
		"github.com/ipfs/go-unixfs":                      "github.com/aliihsank/boxo/ipld/unixfs",
		"github.com/ipfs/go-pinning-service-http-client": "github.com/aliihsank/boxo/pinning/remote/client",
		"github.com/ipfs/go-path":                        "github.com/aliihsank/boxo/path",
		"github.com/ipfs/go-namesys":                     "github.com/aliihsank/boxo/namesys",
		"github.com/ipfs/go-mfs":                         "github.com/aliihsank/boxo/mfs",
		"github.com/ipfs/go-ipfs-provider":               "github.com/aliihsank/boxo/provider",
		"github.com/ipfs/go-ipfs-pinner":                 "github.com/aliihsank/boxo/pinning/pinner",
		"github.com/ipfs/go-ipfs-keystore":               "github.com/aliihsank/boxo/keystore",
		"github.com/ipfs/go-filestore":                   "github.com/aliihsank/boxo/filestore",
		"github.com/ipfs/go-ipns":                        "github.com/aliihsank/boxo/ipns",
		"github.com/ipfs/go-blockservice":                "github.com/aliihsank/boxo/blockservice",
		"github.com/ipfs/go-ipfs-chunker":                "github.com/aliihsank/boxo/chunker",
		"github.com/ipfs/go-fetcher":                     "github.com/aliihsank/boxo/fetcher",
		"github.com/ipfs/go-ipfs-blockstore":             "github.com/aliihsank/boxo/blockstore",
		"github.com/ipfs/go-ipfs-posinfo":                "github.com/aliihsank/boxo/filestore/posinfo",
		"github.com/ipfs/go-ipfs-util":                   "github.com/aliihsank/boxo/util",
		"github.com/ipfs/go-ipfs-ds-help":                "github.com/aliihsank/boxo/datastore/dshelp",
		"github.com/ipfs/go-verifcid":                    "github.com/aliihsank/boxo/verifcid",
		"github.com/ipfs/go-ipfs-exchange-offline":       "github.com/aliihsank/boxo/exchange/offline",
		"github.com/ipfs/go-ipfs-routing":                "github.com/aliihsank/boxo/routing",
		"github.com/ipfs/go-ipfs-exchange-interface":     "github.com/aliihsank/boxo/exchange",
		"github.com/ipfs/go-merkledag":                   "github.com/aliihsank/boxo/ipld/merkledag",
		"github.com/boxo/ipld/car":                       "github.com/ipld/go-car",

		// Pre Boxo rename
		"github.com/ipfs/go-libipfs/gateway":               "github.com/aliihsank/boxo/gateway",
		"github.com/ipfs/go-libipfs/bitswap":               "github.com/aliihsank/boxo/bitswap",
		"github.com/ipfs/go-libipfs/files":                 "github.com/aliihsank/boxo/files",
		"github.com/ipfs/go-libipfs/tar":                   "github.com/aliihsank/boxo/tar",
		"github.com/ipfs/go-libipfs/coreiface":             "github.com/aliihsank/boxo/coreiface",
		"github.com/ipfs/go-libipfs/unixfs":                "github.com/aliihsank/boxo/ipld/unixfs",
		"github.com/ipfs/go-libipfs/pinning/remote/client": "github.com/aliihsank/boxo/pinning/remote/client",
		"github.com/ipfs/go-libipfs/path":                  "github.com/aliihsank/boxo/path",
		"github.com/ipfs/go-libipfs/namesys":               "github.com/aliihsank/boxo/namesys",
		"github.com/ipfs/go-libipfs/mfs":                   "github.com/aliihsank/boxo/mfs",
		"github.com/ipfs/go-libipfs/provider":              "github.com/aliihsank/boxo/provider",
		"github.com/ipfs/go-libipfs/pinning/pinner":        "github.com/aliihsank/boxo/pinning/pinner",
		"github.com/ipfs/go-libipfs/keystore":              "github.com/aliihsank/boxo/keystore",
		"github.com/ipfs/go-libipfs/filestore":             "github.com/aliihsank/boxo/filestore",
		"github.com/ipfs/go-libipfs/ipns":                  "github.com/aliihsank/boxo/ipns",
		"github.com/ipfs/go-libipfs/blockservice":          "github.com/aliihsank/boxo/blockservice",
		"github.com/ipfs/go-libipfs/chunker":               "github.com/aliihsank/boxo/chunker",
		"github.com/ipfs/go-libipfs/fetcher":               "github.com/aliihsank/boxo/fetcher",
		"github.com/ipfs/go-libipfs/blockstore":            "github.com/aliihsank/boxo/blockstore",
		"github.com/ipfs/go-libipfs/filestore/posinfo":     "github.com/aliihsank/boxo/filestore/posinfo",
		"github.com/ipfs/go-libipfs/util":                  "github.com/aliihsank/boxo/util",
		"github.com/ipfs/go-libipfs/datastore/dshelp":      "github.com/aliihsank/boxo/datastore/dshelp",
		"github.com/ipfs/go-libipfs/verifcid":              "github.com/aliihsank/boxo/verifcid",
		"github.com/ipfs/go-libipfs/exchange/offline":      "github.com/aliihsank/boxo/exchange/offline",
		"github.com/ipfs/go-libipfs/routing":               "github.com/aliihsank/boxo/routing",
		"github.com/ipfs/go-libipfs/exchange":              "github.com/aliihsank/boxo/exchange",

		// Unmigrated things
		"github.com/ipfs/go-libipfs/blocks": "github.com/ipfs/go-block-format",
		"github.com/aliihsank/boxo/blocks":       "github.com/ipfs/go-block-format",
	},
	Modules: []string{
		"github.com/ipfs/go-bitswap",
		"github.com/ipfs/go-ipfs-files",
		"github.com/ipfs/tar-utils",
		"gihtub.com/ipfs/go-block-format",
		"github.com/ipfs/interface-go-ipfs-core",
		"github.com/ipfs/go-unixfs",
		"github.com/ipfs/go-pinning-service-http-client",
		"github.com/ipfs/go-path",
		"github.com/ipfs/go-namesys",
		"github.com/ipfs/go-mfs",
		"github.com/ipfs/go-ipfs-provider",
		"github.com/ipfs/go-ipfs-pinner",
		"github.com/ipfs/go-ipfs-keystore",
		"github.com/ipfs/go-filestore",
		"github.com/ipfs/go-ipns",
		"github.com/ipfs/go-blockservice",
		"github.com/ipfs/go-ipfs-chunker",
		"github.com/ipfs/go-fetcher",
		"github.com/ipfs/go-ipfs-blockstore",
		"github.com/ipfs/go-ipfs-posinfo",
		"github.com/ipfs/go-ipfs-util",
		"github.com/ipfs/go-ipfs-ds-help",
		"github.com/ipfs/go-verifcid",
		"github.com/ipfs/go-ipfs-exchange-offline",
		"github.com/ipfs/go-ipfs-routing",
		"github.com/ipfs/go-ipfs-exchange-interface",
		"github.com/ipfs/go-libipfs",
	},
}

func ReadConfig(r io.Reader) (Config, error) {
	var config Config
	err := json.NewDecoder(r).Decode(&config)
	if err != nil {
		return Config{}, fmt.Errorf("reading and decoding config: %w", err)
	}
	return config, nil
}
