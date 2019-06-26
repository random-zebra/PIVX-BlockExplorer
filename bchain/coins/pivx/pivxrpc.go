package pivx

import (
	"blockbook/bchain"
	"blockbook/bchain/coins/btc"
	"encoding/json"

	"github.com/golang/glog"
)

// PivXRPC is an interface to JSON-RPC bitcoind service.
type PivXRPC struct {
	*btc.BitcoinRPC
    BitcoinGetChainInfo func() (*bchain.ChainInfo, error)
}

// NewPivXRPC returns new PivXRPC instance.
func NewPivXRPC(config json.RawMessage, pushHandler func(bchain.NotificationType)) (bchain.BlockChain, error) {
	b, err := btc.NewBitcoinRPC(config, pushHandler)
	if err != nil {
		return nil, err
	}

	s := &PivXRPC{
		b.(*btc.BitcoinRPC),
        b.GetChainInfo,
	}
	s.RPCMarshaler = btc.JSONMarshalerV1{}
	s.ChainConfig.SupportsEstimateFee = true
	s.ChainConfig.SupportsEstimateSmartFee = false

	return s, nil
}

// Initialize initializes PivXRPC instance.
func (b *PivXRPC) Initialize() error {
	ci, err := b.GetChainInfo()
	if err != nil {
		return err
	}
	chainName := ci.Chain

	glog.Info("Chain name ", chainName)
	params := GetChainParams(chainName)

	// always create parser
	b.Parser = NewPivXParser(params, b.ChainConfig)

	// parameters for getInfo request
	if params.Net == MainnetMagic {
		b.Testnet = false
		b.Network = "livenet"
	} else {
		b.Testnet = true
		b.Network = "testnet"
	}

	glog.Info("rpc: block chain ", params.Name)

	return nil
}


// getinfo

type CmdGetInfo struct {
	Method string `json:"method"`
}

type ResGetInfo struct {
	Error  *bchain.RPCError `json:"error"`
	Result struct {
        MoneySupply   json.Number `json:"moneysupply"`
        ZerocoinSupply  bchain.ZCdenoms    `json:"zPIVsupply"`
	} `json:"result"`
}

// getmasternodecount

type CmdGetMasternodeCount struct {
	Method string `json:"method"`
}

type ResGetMasternodeCount struct {
	Error  *bchain.RPCError `json:"error"`
    Result struct {
        Total   int    `json:"total"`
        Stable   int    `json:"stable"`
        Enabled   int    `json:"enabled"`
        InQueue   int    `json:"inqueue"`
	} `json:"result"`
}

// GetNextSuperBlock returns the next superblock height after nHeight
func (b *PivXRPC) GetNextSuperBlock(nHeight int) int {
    nBlocksPerPeriod := 43200
    if b.Testnet {
        nBlocksPerPeriod = 144
    }
    return nHeight - nHeight % nBlocksPerPeriod + nBlocksPerPeriod
}

// GetChainInfo returns information about the connected backend
// PIVX adds MoneySupply and ZerocoinSupply to btc implementation
func (b *PivXRPC) GetChainInfo() (*bchain.ChainInfo, error) {
    rv, err := b.BitcoinGetChainInfo()
    if err != nil {
        return nil, err
    }

	glog.V(1).Info("rpc: getinfo")

    resGi := ResGetInfo{}
    err = b.Call(&CmdGetInfo{Method: "getinfo"}, &resGi)
    if err != nil {
        return nil, err
    }
    if resGi.Error != nil {
        return nil, resGi.Error
    }
    rv.MoneySupply = resGi.Result.MoneySupply
    rv.ZerocoinSupply = resGi.Result.ZerocoinSupply

    glog.V(1).Info("rpc: getmasternodecount")

    resMc := ResGetMasternodeCount{}
    err = b.Call(&CmdGetMasternodeCount{Method: "getmasternodecount"}, &resMc)
    if err != nil {
        return nil, err
    }
    if resMc.Error != nil {
        return nil, resMc.Error
    }
    rv.MasternodeCount = resMc.Result.Enabled

    rv.NextSuperBlock = b.GetNextSuperBlock(rv.Headers)

	return rv, nil
}

// findserial
type CmdFindSerial struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
}

type ResFindSerial struct {
	Error  *bchain.RPCError `json:"error"`
	Result struct {
		Success bool      `json:"success"`
		Txid    string    `json:"txid"`
	} `json:"result"`
}

func (b *PivXRPC) Findzcserial(serialHex string) (string, error) {
    glog.V(1).Info("rpc: findserial")

	res := ResFindSerial{}
	req := CmdFindSerial{Method: "findserial"}
	req.Params = []string{serialHex}
	err := b.Call(&req, &res)

	if err != nil {
		return "", err
	}
	if res.Error != nil {
		return "", res.Error
	}
    if !res.Result.Success {
		return "Serial not found in blockchain", nil
	}
	return res.Result.Txid, nil
}
