package api

import (
	"blockbook/bchain"
	"blockbook/bchain/coins/eth"
	"blockbook/common"
	"blockbook/db"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/juju/errors"
)

// Worker is handle to api worker
type Worker struct {
	db          *db.RocksDB
	txCache     *db.TxCache
	chain       bchain.BlockChain
	chainParser bchain.BlockChainParser
	chainType   bchain.ChainType
	mempool     bchain.Mempool
	is          *common.InternalState
}

// NewWorker creates new api worker
func NewWorker(db *db.RocksDB, chain bchain.BlockChain, mempool bchain.Mempool, txCache *db.TxCache, is *common.InternalState) (*Worker, error) {
	w := &Worker{
		db:          db,
		txCache:     txCache,
		chain:       chain,
		chainParser: chain.GetChainParser(),
		chainType:   chain.GetChainParser().GetChainType(),
		mempool:     mempool,
		is:          is,
	}
	return w, nil
}

func (w *Worker) getAddressesFromVout(vout *bchain.Vout) (bchain.AddressDescriptor, []string, bool, error) {
	addrDesc, err := w.chainParser.GetAddrDescFromVout(vout)
	if err != nil {
		return nil, nil, false, err
	}
	a, s, err := w.chainParser.GetAddressesFromAddrDesc(addrDesc)
	return addrDesc, a, s, err
}

// setSpendingTxToVout is helper function, that finds transaction that spent given output and sets it to the output
// there is no direct index for the operation, it must be found using addresses -> txaddresses -> tx
func (w *Worker) setSpendingTxToVout(vout *Vout, txid string, height uint32) error {
	err := w.db.GetAddrDescTransactions(vout.AddrDesc, height, maxUint32, func(t string, height uint32, indexes []int32) error {
		for _, index := range indexes {
			// take only inputs
			if index < 0 {
				index = ^index
				tsp, err := w.db.GetTxAddresses(t)
				if err != nil {
					return err
				} else if tsp == nil {
					glog.Warning("DB inconsistency:  tx ", t, ": not found in txAddresses")
				} else if len(tsp.Inputs) > int(index) {
					if tsp.Inputs[index].ValueSat.Cmp((*big.Int)(vout.ValueSat)) == 0 {
						spentTx, spentHeight, err := w.txCache.GetTransaction(t)
						if err != nil {
							glog.Warning("Tx ", t, ": not found")
						} else {
							if len(spentTx.Vin) > int(index) {
								if spentTx.Vin[index].Txid == txid {
									vout.SpentTxID = t
									vout.SpentHeight = int(spentHeight)
									vout.SpentIndex = int(index)
									return &db.StopIteration{}
								}
							}
						}
					}
				}
			}
		}
		return nil
	})
	return err
}

// GetSpendingTxid returns transaction id of transaction that spent given output
func (w *Worker) GetSpendingTxid(txid string, n int) (string, error) {
	start := time.Now()
	tx, err := w.GetTransaction(txid, false, false)
	if err != nil {
		return "", err
	}
	if n >= len(tx.Vout) || n < 0 {
		return "", NewAPIError(fmt.Sprintf("Passed incorrect vout index %v for tx %v, len vout %v", n, tx.Txid, len(tx.Vout)), false)
	}
	err = w.setSpendingTxToVout(&tx.Vout[n], tx.Txid, uint32(tx.Blockheight))
	if err != nil {
		return "", err
	}
	glog.Info("GetSpendingTxid ", txid, " ", n, " finished in ", time.Since(start))
	return tx.Vout[n].SpentTxID, nil
}

// GetTransaction reads transaction data from txid
func (w *Worker) GetTransaction(txid string, spendingTxs bool, specificJSON bool) (*Tx, error) {
	bchainTx, height, err := w.txCache.GetTransaction(txid)
	if err != nil {
		if err == bchain.ErrTxNotFound {
			return nil, NewAPIError(fmt.Sprintf("Transaction '%v' not found", txid), true)
		}
		return nil, NewAPIError(fmt.Sprintf("Transaction '%v' not found (%v)", txid, err), true)
	}
	return w.GetTransactionFromBchainTx(bchainTx, height, spendingTxs, specificJSON)
}

// GetTransactionFromBchainTx reads transaction data from txid
func (w *Worker) GetTransactionFromBchainTx(bchainTx *bchain.Tx, height uint32, spendingTxs bool, specificJSON bool) (*Tx, error) {
	var err error
	var ta *db.TxAddresses
	var tokens []TokenTransfer
	var ethSpecific *EthereumSpecific
	var blockhash string
	if bchainTx.Confirmations > 0 {
		if w.chainType == bchain.ChainBitcoinType {
			ta, err = w.db.GetTxAddresses(bchainTx.Txid)
			if err != nil {
				return nil, errors.Annotatef(err, "GetTxAddresses %v", bchainTx.Txid)
			}
		}
		blockhash, err = w.db.GetBlockHash(height)
		if err != nil {
			return nil, errors.Annotatef(err, "GetBlockHash %v", height)
		}
	}
	var valInSat, valOutSat, feesSat big.Int
	var pValInSat *big.Int
	vins := make([]Vin, len(bchainTx.Vin))
	for i := range bchainTx.Vin {
		bchainVin := &bchainTx.Vin[i]
		vin := &vins[i]
		vin.Txid = bchainVin.Txid
		vin.N = i
		vin.Vout = bchainVin.Vout
		vin.Sequence = int64(bchainVin.Sequence)
		vin.Hex = bchainVin.ScriptSig.Hex
		vin.Coinbase = bchainVin.Coinbase
		if w.chainType == bchain.ChainBitcoinType {
			//  bchainVin.Txid=="" is coinbase transaction
			if bchainVin.Txid != "" {
				// load spending addresses from TxAddresses
				tas, err := w.db.GetTxAddresses(bchainVin.Txid)
				if err != nil {
					return nil, errors.Annotatef(err, "GetTxAddresses %v", bchainVin.Txid)
				}
				if tas == nil {
					// try to load from backend
					otx, _, err := w.txCache.GetTransaction(bchainVin.Txid)
					if err != nil {
						if err == bchain.ErrTxNotFound {
							// try to get AddrDesc using coin specific handling and continue processing the tx
							vin.AddrDesc = w.chainParser.GetAddrDescForUnknownInput(bchainTx, i)
                            vin.ValueSat = (*Amount)(w.chainParser.GetValueSatForUnknownInput(bchainTx, i))
                            if vin.ValueSat != nil {
                                valInSat.Add(&valInSat, (*big.Int)(vin.ValueSat))
                            }
							vin.Addresses, vin.Searchable, err = w.chainParser.GetAddressesFromAddrDesc(vin.AddrDesc)
							if err != nil {
								glog.Warning("GetAddressesFromAddrDesc tx ", bchainVin.Txid, ", addrDesc ", vin.AddrDesc, ": ", err)
							}
							continue
						}
						return nil, errors.Annotatef(err, "txCache.GetTransaction %v", bchainVin.Txid)
					}
					// mempool transactions are not in TxAddresses but confirmed should be there, log a problem
					if bchainTx.Confirmations > 0 {
						inSync, _, _ := w.is.GetSyncState()
						// backend can report tx as confirmed, however blockbook is still syncing (!inSync), in this case do not log a problem
						if bchainTx.Confirmations != 1 || inSync {
							glog.Warning("DB inconsistency:  tx ", bchainVin.Txid, ": not found in txAddresses")
						}
					}
					if len(otx.Vout) > int(vin.Vout) {
						vout := &otx.Vout[vin.Vout]
						vin.ValueSat = (*Amount)(&vout.ValueSat)
						vin.AddrDesc, vin.Addresses, vin.Searchable, err = w.getAddressesFromVout(vout)
						if err != nil {
							glog.Errorf("getAddressesFromVout error %v, vout %+v", err, vout)
						}
					}
				} else {
					if len(tas.Outputs) > int(vin.Vout) {
						output := &tas.Outputs[vin.Vout]
						vin.ValueSat = (*Amount)(&output.ValueSat)
						vin.AddrDesc = output.AddrDesc
						vin.Addresses, vin.Searchable, err = output.Addresses(w.chainParser)
						if err != nil {
							glog.Errorf("output.Addresses error %v, tx %v, output %v", err, bchainVin.Txid, i)
						}
					}
				}
				if vin.ValueSat != nil {
					valInSat.Add(&valInSat, (*big.Int)(vin.ValueSat))
				}
			}
		} else if w.chainType == bchain.ChainEthereumType {
			if len(bchainVin.Addresses) > 0 {
				vin.AddrDesc, err = w.chainParser.GetAddrDescFromAddress(bchainVin.Addresses[0])
				if err != nil {
					glog.Errorf("GetAddrDescFromAddress error %v, tx %v, bchainVin %v", err, bchainTx.Txid, bchainVin)
				}
				vin.Addresses = bchainVin.Addresses
				vin.Searchable = true
			}
		}
	}
	vouts := make([]Vout, len(bchainTx.Vout))
	for i := range bchainTx.Vout {
		bchainVout := &bchainTx.Vout[i]
		vout := &vouts[i]
		vout.N = i
		vout.ValueSat = (*Amount)(&bchainVout.ValueSat)
		valOutSat.Add(&valOutSat, &bchainVout.ValueSat)
		vout.Hex = bchainVout.ScriptPubKey.Hex
		vout.AddrDesc, vout.Addresses, vout.Searchable, err = w.getAddressesFromVout(bchainVout)
		if err != nil {
			glog.V(2).Infof("getAddressesFromVout error %v, %v, output %v", err, bchainTx.Txid, bchainVout.N)
		}
		if ta != nil {
			vout.Spent = ta.Outputs[i].Spent
			if spendingTxs && vout.Spent {
				err = w.setSpendingTxToVout(vout, bchainTx.Txid, height)
				if err != nil {
					glog.Errorf("setSpendingTxToVout error %v, %v, output %v", err, vout.AddrDesc, vout.N)
				}
			}
		}
	}
	if w.chainType == bchain.ChainBitcoinType {
		// for coinbase transactions valIn is 0
		feesSat.Sub(&valInSat, &valOutSat)
		if feesSat.Sign() == -1 {
			feesSat.SetUint64(0)
		}
		pValInSat = &valInSat
	} else if w.chainType == bchain.ChainEthereumType {
		ets, err := w.chainParser.EthereumTypeGetErc20FromTx(bchainTx)
		if err != nil {
			glog.Errorf("GetErc20FromTx error %v, %v", err, bchainTx)
		}
		tokens = make([]TokenTransfer, len(ets))
		for i := range ets {
			e := &ets[i]
			cd, err := w.chainParser.GetAddrDescFromAddress(e.Contract)
			if err != nil {
				glog.Errorf("GetAddrDescFromAddress error %v, contract %v", err, e.Contract)
				continue
			}
			erc20c, err := w.chain.EthereumTypeGetErc20ContractInfo(cd)
			if err != nil {
				glog.Errorf("GetErc20ContractInfo error %v, contract %v", err, e.Contract)
			}
			if erc20c == nil {
				erc20c = &bchain.Erc20Contract{Name: e.Contract}
			}
			tokens[i] = TokenTransfer{
				Type:     ERC20TokenType,
				Token:    e.Contract,
				From:     e.From,
				To:       e.To,
				Decimals: erc20c.Decimals,
				Value:    (*Amount)(&e.Tokens),
				Name:     erc20c.Name,
				Symbol:   erc20c.Symbol,
			}
		}
		ethTxData := eth.GetEthereumTxData(bchainTx)
		// mempool txs do not have fees yet
		if ethTxData.GasUsed != nil {
			feesSat.Mul(ethTxData.GasPrice, ethTxData.GasUsed)
		}
		if len(bchainTx.Vout) > 0 {
			valOutSat = bchainTx.Vout[0].ValueSat
		}
		ethSpecific = &EthereumSpecific{
			GasLimit: ethTxData.GasLimit,
			GasPrice: (*Amount)(ethTxData.GasPrice),
			GasUsed:  ethTxData.GasUsed,
			Nonce:    ethTxData.Nonce,
			Status:   ethTxData.Status,
		}
	}
	// for now do not return size, we would have to compute vsize of segwit transactions
	// size:=len(bchainTx.Hex) / 2
	var sj json.RawMessage
	if specificJSON {
		sj, err = w.chain.GetTransactionSpecific(bchainTx)
		if err != nil {
			return nil, err
		}
	}
	// for mempool transaction get first seen time
	if bchainTx.Confirmations == 0 {
		bchainTx.Blocktime = int64(w.mempool.GetTransactionTime(bchainTx.Txid))
	}
	r := &Tx{
		Blockhash:        blockhash,
		Blockheight:      int(height),
		Blocktime:        bchainTx.Blocktime,
		Confirmations:    bchainTx.Confirmations,
		FeesSat:          (*Amount)(&feesSat),
		Locktime:         bchainTx.LockTime,
		Txid:             bchainTx.Txid,
		ValueInSat:       (*Amount)(pValInSat),
		ValueOutSat:      (*Amount)(&valOutSat),
		Version:          bchainTx.Version,
		Hex:              bchainTx.Hex,
		Vin:              vins,
		Vout:             vouts,
		CoinSpecificData: bchainTx.CoinSpecificData,
		CoinSpecificJSON: sj,
		TokenTransfers:   tokens,
		EthereumSpecific: ethSpecific,
	}
	return r, nil
}

func (w *Worker) getAddressTxids(addrDesc bchain.AddressDescriptor, mempool bool, filter *AddressFilter, maxResults int) ([]string, error) {
	var err error
	txids := make([]string, 0, 4)
	var callback db.GetTransactionsCallback
	if filter.Vout == AddressFilterVoutOff {
		callback = func(txid string, height uint32, indexes []int32) error {
			txids = append(txids, txid)
			if len(txids) >= maxResults {
				return &db.StopIteration{}
			}
			return nil
		}
	} else {
		callback = func(txid string, height uint32, indexes []int32) error {
			for _, index := range indexes {
				vout := index
				if vout < 0 {
					vout = ^vout
				}
				if (filter.Vout == AddressFilterVoutInputs && index < 0) ||
					(filter.Vout == AddressFilterVoutOutputs && index >= 0) ||
					(vout == int32(filter.Vout)) {
					txids = append(txids, txid)
					if len(txids) >= maxResults {
						return &db.StopIteration{}
					}
					break
				}
			}
			return nil
		}
	}
	if mempool {
		uniqueTxs := make(map[string]struct{})
		o, err := w.mempool.GetAddrDescTransactions(addrDesc)
		if err != nil {
			return nil, err
		}
		for _, m := range o {
			if _, found := uniqueTxs[m.Txid]; !found {
				l := len(txids)
				callback(m.Txid, 0, []int32{m.Vout})
				if len(txids) > l {
					uniqueTxs[m.Txid] = struct{}{}
				}
			}
		}
	} else {
		to := filter.ToHeight
		if to == 0 {
			to = maxUint32
		}
		err = w.db.GetAddrDescTransactions(addrDesc, filter.FromHeight, to, callback)
		if err != nil {
			return nil, err
		}
	}
	return txids, nil
}

func (t *Tx) getAddrVoutValue(addrDesc bchain.AddressDescriptor) *big.Int {
	var val big.Int
	for _, vout := range t.Vout {
		if bytes.Equal(vout.AddrDesc, addrDesc) && vout.ValueSat != nil {
			val.Add(&val, (*big.Int)(vout.ValueSat))
		}
	}
	return &val
}

func (t *Tx) getAddrVinValue(addrDesc bchain.AddressDescriptor) *big.Int {
	var val big.Int
	for _, vin := range t.Vin {
		if bytes.Equal(vin.AddrDesc, addrDesc) && vin.ValueSat != nil {
			val.Add(&val, (*big.Int)(vin.ValueSat))
		}
	}
	return &val
}

// GetUniqueTxids removes duplicate transactions
func GetUniqueTxids(txids []string) []string {
	ut := make([]string, len(txids))
	txidsMap := make(map[string]struct{})
	i := 0
	for _, txid := range txids {
		_, e := txidsMap[txid]
		if !e {
			ut[i] = txid
			i++
			txidsMap[txid] = struct{}{}
		}
	}
	return ut[0:i]
}

func (w *Worker) txFromTxAddress(txid string, ta *db.TxAddresses, bi *db.BlockInfo, bestheight uint32) *Tx {
	var err error
	var valInSat, valOutSat, feesSat big.Int
	vins := make([]Vin, len(ta.Inputs))
	for i := range ta.Inputs {
		tai := &ta.Inputs[i]
		vin := &vins[i]
		vin.N = i
		vin.ValueSat = (*Amount)(&tai.ValueSat)
		valInSat.Add(&valInSat, &tai.ValueSat)
		vin.Addresses, vin.Searchable, err = tai.Addresses(w.chainParser)
		if err != nil {
			glog.Errorf("tai.Addresses error %v, tx %v, input %v, tai %+v", err, txid, i, tai)
		}
	}
	vouts := make([]Vout, len(ta.Outputs))
	for i := range ta.Outputs {
		tao := &ta.Outputs[i]
		vout := &vouts[i]
		vout.N = i
		vout.ValueSat = (*Amount)(&tao.ValueSat)
		valOutSat.Add(&valOutSat, &tao.ValueSat)
		vout.Addresses, vout.Searchable, err = tao.Addresses(w.chainParser)
		if err != nil {
			glog.Errorf("tai.Addresses error %v, tx %v, output %v, tao %+v", err, txid, i, tao)
		}
		vout.Spent = tao.Spent
	}
	// for coinbase transactions valIn is 0
	feesSat.Sub(&valInSat, &valOutSat)
	if feesSat.Sign() == -1 {
		feesSat.SetUint64(0)
	}
	r := &Tx{
		Blockhash:     bi.Hash,
		Blockheight:   int(ta.Height),
		Blocktime:     bi.Time,
		Confirmations: bestheight - ta.Height + 1,
		FeesSat:       (*Amount)(&feesSat),
		Txid:          txid,
		ValueInSat:    (*Amount)(&valInSat),
		ValueOutSat:   (*Amount)(&valOutSat),
		Vin:           vins,
		Vout:          vouts,
	}
	return r
}

func computePaging(count, page, itemsOnPage int) (Paging, int, int, int) {
	from := page * itemsOnPage
	totalPages := (count - 1) / itemsOnPage
	if totalPages < 0 {
		totalPages = 0
	}
	if from >= count {
		page = totalPages
	}
	from = page * itemsOnPage
	to := (page + 1) * itemsOnPage
	if to > count {
		to = count
	}
	return Paging{
		ItemsOnPage: itemsOnPage,
		Page:        page + 1,
		TotalPages:  totalPages + 1,
	}, from, to, page
}

func (w *Worker) getEthereumTypeAddressBalances(addrDesc bchain.AddressDescriptor, details AccountDetails, filter *AddressFilter) (*db.AddrBalance, []Token, *bchain.Erc20Contract, uint64, int, int, error) {
	var (
		ba             *db.AddrBalance
		tokens         []Token
		ci             *bchain.Erc20Contract
		n              uint64
		nonContractTxs int
	)
	// unknown number of results for paging
	totalResults := -1
	ca, err := w.db.GetAddrDescContracts(addrDesc)
	if err != nil {
		return nil, nil, nil, 0, 0, 0, NewAPIError(fmt.Sprintf("Address not found, %v", err), true)
	}
	b, err := w.chain.EthereumTypeGetBalance(addrDesc)
	if err != nil {
		return nil, nil, nil, 0, 0, 0, errors.Annotatef(err, "EthereumTypeGetBalance %v", addrDesc)
	}
	if ca != nil {
		ba = &db.AddrBalance{
			Txs: uint32(ca.TotalTxs),
		}
		if b != nil {
			ba.BalanceSat = *b
		}
		n, err = w.chain.EthereumTypeGetNonce(addrDesc)
		if err != nil {
			return nil, nil, nil, 0, 0, 0, errors.Annotatef(err, "EthereumTypeGetNonce %v", addrDesc)
		}
		var filterDesc bchain.AddressDescriptor
		if filter.Contract != "" {
			filterDesc, err = w.chainParser.GetAddrDescFromAddress(filter.Contract)
			if err != nil {
				return nil, nil, nil, 0, 0, 0, NewAPIError(fmt.Sprintf("Invalid contract filter, %v", err), true)
			}
		}
		if details > AccountDetailsBasic {
			tokens = make([]Token, len(ca.Contracts))
			var j int
			for i, c := range ca.Contracts {
				if len(filterDesc) > 0 {
					if !bytes.Equal(filterDesc, c.Contract) {
						continue
					}
					// filter only transactions of this contract
					filter.Vout = i + 1
				}
				validContract := true
				ci, err := w.chain.EthereumTypeGetErc20ContractInfo(c.Contract)
				if err != nil {
					return nil, nil, nil, 0, 0, 0, errors.Annotatef(err, "EthereumTypeGetErc20ContractInfo %v", c.Contract)
				}
				if ci == nil {
					ci = &bchain.Erc20Contract{}
					addresses, _, _ := w.chainParser.GetAddressesFromAddrDesc(c.Contract)
					if len(addresses) > 0 {
						ci.Contract = addresses[0]
						ci.Name = addresses[0]
					}
					validContract = false
				}
				// do not read contract balances etc in case of Basic option
				if details >= AccountDetailsTokenBalances && validContract {
					b, err = w.chain.EthereumTypeGetErc20ContractBalance(addrDesc, c.Contract)
					if err != nil {
						// return nil, nil, nil, errors.Annotatef(err, "EthereumTypeGetErc20ContractBalance %v %v", addrDesc, c.Contract)
						glog.Warningf("EthereumTypeGetErc20ContractBalance addr %v, contract %v, %v", addrDesc, c.Contract, err)
					}
				} else {
					b = nil
				}
				tokens[j] = Token{
					Type:          ERC20TokenType,
					BalanceSat:    (*Amount)(b),
					Contract:      ci.Contract,
					Name:          ci.Name,
					Symbol:        ci.Symbol,
					Transfers:     int(c.Txs),
					Decimals:      ci.Decimals,
					ContractIndex: strconv.Itoa(i + 1),
				}
				j++
			}
			tokens = tokens[:j]
		}
		ci, err = w.chain.EthereumTypeGetErc20ContractInfo(addrDesc)
		if err != nil {
			return nil, nil, nil, 0, 0, 0, err
		}
		if filter.FromHeight == 0 && filter.ToHeight == 0 {
			// compute total results for paging
			if filter.Vout == AddressFilterVoutOff {
				totalResults = int(ca.TotalTxs)
			} else if filter.Vout == 0 {
				totalResults = int(ca.NonContractTxs)
			} else if filter.Vout > 0 && filter.Vout-1 < len(ca.Contracts) {
				totalResults = int(ca.Contracts[filter.Vout-1].Txs)
			}
		}
		nonContractTxs = int(ca.NonContractTxs)
	} else {
		// addresses without any normal transactions can have internal transactions and therefore balance
		if b != nil {
			ba = &db.AddrBalance{
				BalanceSat: *b,
			}
		}
	}
	return ba, tokens, ci, n, nonContractTxs, totalResults, nil
}

func (w *Worker) txFromTxid(txid string, bestheight uint32, option AccountDetails, blockInfo *db.BlockInfo) (*Tx, error) {
	var tx *Tx
	var err error
	// only ChainBitcoinType supports TxHistoryLight
	if option == AccountDetailsTxHistoryLight && w.chainType == bchain.ChainBitcoinType {
		ta, err := w.db.GetTxAddresses(txid)
		if err != nil {
			return nil, errors.Annotatef(err, "GetTxAddresses %v", txid)
		}
		if ta == nil {
			glog.Warning("DB inconsistency:  tx ", txid, ": not found in txAddresses")
			// as fallback, get tx from backend
			tx, err = w.GetTransaction(txid, false, true)
			if err != nil {
				return nil, errors.Annotatef(err, "GetTransaction %v", txid)
			}
		} else {
			if blockInfo == nil {
				blockInfo, err = w.db.GetBlockInfo(ta.Height)
				if err != nil {
					return nil, errors.Annotatef(err, "GetBlockInfo %v", ta.Height)
				}
				if blockInfo == nil {
					glog.Warning("DB inconsistency:  block height ", ta.Height, ": not found in db")
					// provide empty BlockInfo to return the rest of tx data
					blockInfo = &db.BlockInfo{}
				}
			}
			tx = w.txFromTxAddress(txid, ta, blockInfo, bestheight)
		}
	} else {
		tx, err = w.GetTransaction(txid, false, true)
		if err != nil {
			return nil, errors.Annotatef(err, "GetTransaction %v", txid)
		}
	}
	return tx, nil
}

func (w *Worker) getAddrDescAndNormalizeAddress(address string) (bchain.AddressDescriptor, string, error) {
	addrDesc, err := w.chainParser.GetAddrDescFromAddress(address)
	if err != nil {
		return nil, "", NewAPIError(fmt.Sprintf("Invalid address, %v", err), true)
	}
	// convert the address to the format defined by the parser
	addresses, _, err := w.chainParser.GetAddressesFromAddrDesc(addrDesc)
	if err != nil {
		glog.V(2).Infof("GetAddressesFromAddrDesc error %v, %v", err, addrDesc)
	}
	if len(addresses) == 1 {
		address = addresses[0]
	}
	return addrDesc, address, nil
}

// GetAddress computes address value and gets transactions for given address
func (w *Worker) GetAddress(address string, page int, txsOnPage int, option AccountDetails, filter *AddressFilter) (*Address, error) {
	start := time.Now()
	page--
	if page < 0 {
		page = 0
	}
	var (
		ba                       *db.AddrBalance
		tokens                   []Token
		erc20c                   *bchain.Erc20Contract
		txm                      []string
		txs                      []*Tx
		txids                    []string
		pg                       Paging
		uBalSat                  big.Int
		totalReceived, totalSent *big.Int
		nonce                    string
		unconfirmedTxs           int
		nonTokenTxs              int
		totalResults             int
	)
	addrDesc, address, err := w.getAddrDescAndNormalizeAddress(address)
	if err != nil {
		return nil, err
	}
	if w.chainType == bchain.ChainEthereumType {
		var n uint64
		ba, tokens, erc20c, n, nonTokenTxs, totalResults, err = w.getEthereumTypeAddressBalances(addrDesc, option, filter)
		if err != nil {
			return nil, err
		}
		nonce = strconv.Itoa(int(n))
	} else {
		// ba can be nil if the address is only in mempool!
		ba, err = w.db.GetAddrDescBalance(addrDesc, db.AddressBalanceDetailNoUTXO)
		if err != nil {
			return nil, NewAPIError(fmt.Sprintf("Address not found, %v", err), true)
		}
		if ba != nil {
			// totalResults is known only if there is no filter
			if filter.Vout == AddressFilterVoutOff && filter.FromHeight == 0 && filter.ToHeight == 0 {
				totalResults = int(ba.Txs)
			} else {
				totalResults = -1
			}
		}
	}
	// if there are only unconfirmed transactions, there is no paging
	if ba == nil {
		ba = &db.AddrBalance{}
		page = 0
	}
	// process mempool, only if toHeight is not specified
	if filter.ToHeight == 0 && !filter.OnlyConfirmed {
		txm, err = w.getAddressTxids(addrDesc, true, filter, maxInt)
		if err != nil {
			return nil, errors.Annotatef(err, "getAddressTxids %v true", addrDesc)
		}
		for _, txid := range txm {
			tx, err := w.GetTransaction(txid, false, false)
			// mempool transaction may fail
			if err != nil || tx == nil {
				glog.Warning("GetTransaction in mempool: ", err)
			} else {
				// skip already confirmed txs, mempool may be out of sync
				if tx.Confirmations == 0 {
					unconfirmedTxs++
					uBalSat.Add(&uBalSat, tx.getAddrVoutValue(addrDesc))
					uBalSat.Sub(&uBalSat, tx.getAddrVinValue(addrDesc))
					if page == 0 {
						if option == AccountDetailsTxidHistory {
							txids = append(txids, tx.Txid)
						} else if option >= AccountDetailsTxHistoryLight {
							txs = append(txs, tx)
						}
					}
				}
			}
		}
	}
	// get tx history if requested by option or check mempool if there are some transactions for a new address
	if option >= AccountDetailsTxidHistory {
		txc, err := w.getAddressTxids(addrDesc, false, filter, (page+1)*txsOnPage)
		if err != nil {
			return nil, errors.Annotatef(err, "getAddressTxids %v false", addrDesc)
		}
		bestheight, _, err := w.db.GetBestBlock()
		if err != nil {
			return nil, errors.Annotatef(err, "GetBestBlock")
		}
		var from, to int
		pg, from, to, page = computePaging(len(txc), page, txsOnPage)
		if len(txc) >= txsOnPage {
			if totalResults < 0 {
				pg.TotalPages = -1
			} else {
				pg, _, _, _ = computePaging(totalResults, page, txsOnPage)
			}
		}
		for i := from; i < to; i++ {
			txid := txc[i]
			if option == AccountDetailsTxidHistory {
				txids = append(txids, txid)
			} else {
				tx, err := w.txFromTxid(txid, bestheight, option, nil)
				if err != nil {
					return nil, err
				}
				txs = append(txs, tx)
			}
		}
	}
	if w.chainType == bchain.ChainBitcoinType {
		totalReceived = ba.ReceivedSat()
		totalSent = &ba.SentSat
	}
	r := &Address{
		Paging:                pg,
		AddrStr:               address,
		BalanceSat:            (*Amount)(&ba.BalanceSat),
		TotalReceivedSat:      (*Amount)(totalReceived),
		TotalSentSat:          (*Amount)(totalSent),
		Txs:                   int(ba.Txs),
		NonTokenTxs:           nonTokenTxs,
		UnconfirmedBalanceSat: (*Amount)(&uBalSat),
		UnconfirmedTxs:        unconfirmedTxs,
		Transactions:          txs,
		Txids:                 txids,
		Tokens:                tokens,
		Erc20Contract:         erc20c,
		Nonce:                 nonce,
	}
	glog.Info("GetAddress ", address, " finished in ", time.Since(start))
	return r, nil
}

func (w *Worker) waitForBackendSync() {
	// wait a short time if blockbook is synchronizing with backend
	inSync, _, _ := w.is.GetSyncState()
	count := 30
	for !inSync && count > 0 {
		time.Sleep(time.Millisecond * 100)
		count--
		inSync, _, _ = w.is.GetSyncState()
	}
}

func (w *Worker) getAddrDescUtxo(addrDesc bchain.AddressDescriptor, ba *db.AddrBalance, onlyConfirmed bool, onlyMempool bool) (Utxos, error) {
	w.waitForBackendSync()
	var err error
	r := make(Utxos, 0, 8)
	spentInMempool := make(map[string]struct{})
	if !onlyConfirmed {
		// get utxo from mempool
		txm, err := w.getAddressTxids(addrDesc, true, &AddressFilter{Vout: AddressFilterVoutOff}, maxInt)
		if err != nil {
			return nil, err
		}
		if len(txm) > 0 {
			mc := make([]*bchain.Tx, len(txm))
			for i, txid := range txm {
				// get mempool txs and process their inputs to detect spends between mempool txs
				bchainTx, _, err := w.txCache.GetTransaction(txid)
				// mempool transaction may fail
				if err != nil {
					glog.Error("GetTransaction in mempool ", txid, ": ", err)
				} else {
					mc[i] = bchainTx
					// get outputs spent by the mempool tx
					for i := range bchainTx.Vin {
						vin := &bchainTx.Vin[i]
						spentInMempool[vin.Txid+strconv.Itoa(int(vin.Vout))] = struct{}{}
					}
				}
			}
			for _, bchainTx := range mc {
				if bchainTx != nil {
					for i := range bchainTx.Vout {
						vout := &bchainTx.Vout[i]
						vad, err := w.chainParser.GetAddrDescFromVout(vout)
						if err == nil && bytes.Equal(addrDesc, vad) {
							// report only outpoints that are not spent in mempool
							_, e := spentInMempool[bchainTx.Txid+strconv.Itoa(i)]
							if !e {
								stakeContract := false
								if len(bchainTx.Vout[i].ScriptPubKey.Addresses) > 1 {
									for _, addrStr := range bchainTx.Vout[i].ScriptPubKey.Addresses {
										prefix := addrStr[0:1]
										if prefix == "S" {
											stakeContract = true
										}
									}
									stakeContract = true
								}
								r = append(r, Utxo{
									Txid:      bchainTx.Txid,
									Vout:      int32(i),
									AmountSat: (*Amount)(&vout.ValueSat),
									Locktime:  bchainTx.LockTime,
									StakeContract: stakeContract,
								})
							}
						}
					}
				}
			}
		}
	}
	if !onlyMempool {
		// get utxo from index
		if ba == nil {
			ba, err = w.db.GetAddrDescBalance(addrDesc, db.AddressBalanceDetailUTXO)
			if err != nil {
				return nil, NewAPIError(fmt.Sprintf("Address not found, %v", err), true)
			}
		}
		// ba can be nil if the address is only in mempool!
		if ba != nil && len(ba.Utxos) > 0 {
			b, _, err := w.db.GetBestBlock()
			if err != nil {
				return nil, err
			}
			bestheight := int(b)
			var checksum big.Int
			checksum.Set(&ba.BalanceSat)
			// go backwards to get the newest first
			for i := len(ba.Utxos) - 1; i >= 0; i-- {
				utxo := &ba.Utxos[i]
				txid, err := w.chainParser.UnpackTxid(utxo.BtxID)
				if err != nil {
					return nil, err
				}
				stakeContract := false
				ta, err := w.db.GetTxAddresses(txid)
				if err != nil {
					return nil, err
				}
				addr, _, err := ta.Outputs[utxo.Vout].Addresses(w.chainParser)
				if err != nil {
					return nil, err
				}
				if len(addr) > 1 {
					for _, addrStr := range addr {
						prefix := addrStr[0:1]
						if prefix == "S" {
							stakeContract = true
						}
					}
				}
				_, e := spentInMempool[txid+strconv.Itoa(int(utxo.Vout))]
				if !e {
					r = append(r, Utxo{
						Txid:          txid,
						Vout:          utxo.Vout,
						AmountSat:     (*Amount)(&utxo.ValueSat),
						Height:        int(utxo.Height),
						Confirmations: bestheight - int(utxo.Height) + 1,
						StakeContract: stakeContract,
					})
				}
				checksum.Sub(&checksum, &utxo.ValueSat)
			}
			if checksum.Uint64() != 0 {
				glog.Warning("DB inconsistency:  ", addrDesc, ": checksum is not zero")
			}
		}
	}
	return r, nil
}

// GetAddressUtxo returns unspent outputs for given address
func (w *Worker) GetAddressUtxo(address string, onlyConfirmed bool) (Utxos, error) {
	if w.chainType != bchain.ChainBitcoinType {
		return nil, NewAPIError("Not supported", true)
	}
	start := time.Now()
	addrDesc, err := w.chainParser.GetAddrDescFromAddress(address)
	if err != nil {
		return nil, NewAPIError(fmt.Sprintf("Invalid address '%v', %v", address, err), true)
	}
	r, err := w.getAddrDescUtxo(addrDesc, nil, onlyConfirmed, false)
	if err != nil {
		return nil, err
	}
	glog.Info("GetAddressUtxo ", address, ", ", len(r), " utxos, finished in ", time.Since(start))
	return r, nil
}

// GetBlocks returns BlockInfo for blocks on given page
func (w *Worker) GetBlocks(page int, blocksOnPage int) (*Blocks, error) {
	start := time.Now()
	page--
	if page < 0 {
		page = 0
	}
	b, _, err := w.db.GetBestBlock()
	bestheight := int(b)
	if err != nil {
		return nil, errors.Annotatef(err, "GetBestBlock")
	}
	pg, from, to, page := computePaging(bestheight+1, page, blocksOnPage)
	r := &Blocks{Paging: pg}
	r.Blocks = make([]db.BlockInfo, to-from)
	for i := from; i < to; i++ {
		bi, err := w.db.GetBlockInfo(uint32(bestheight - i))
		if err != nil {
			return nil, err
		}
		if bi == nil {
			r.Blocks = r.Blocks[:i]
			break
		}
		r.Blocks[i-from] = *bi
	}
	glog.Info("GetBlocks page ", page, " finished in ", time.Since(start))
	return r, nil
}

// GetBlock returns paged data about block
func (w *Worker) GetBlock(bid string, page int, txsOnPage int) (*Block, error) {
	start := time.Now()
	page--
	if page < 0 {
		page = 0
	}
	// try to decide if passed string (bid) is block height or block hash
	// if it's a number, must be less than int32
	var hash string
	height, err := strconv.Atoi(bid)
	if err == nil && height < int(maxUint32) {
		hash, err = w.db.GetBlockHash(uint32(height))
		if err != nil {
			hash = bid
		}
	} else {
		hash = bid
	}
	if hash == "" {
		return nil, NewAPIError("Block not found", true)
	}
	bi, err := w.chain.GetBlockInfo(hash)
	if err != nil {
		if err == bchain.ErrBlockNotFound {
			return nil, NewAPIError("Block not found", true)
		}
		return nil, NewAPIError(fmt.Sprintf("Block not found, %v", err), true)
	}
	dbi := &db.BlockInfo{
		Hash:   bi.Hash,
		Height: bi.Height,
		Time:   bi.Time,
	}
	txCount := len(bi.Txids)
	bestheight, _, err := w.db.GetBestBlock()
	if err != nil {
		return nil, errors.Annotatef(err, "GetBestBlock")
	}
	pg, from, to, page := computePaging(txCount, page, txsOnPage)
	txs := make([]*Tx, to-from)
	txi := 0
	for i := from; i < to; i++ {
		txs[txi], err = w.txFromTxid(bi.Txids[i], bestheight, AccountDetailsTxHistoryLight, dbi)
		if err != nil {
			return nil, err
		}
		txi++
	}
	if bi.Prev == "" && bi.Height != 0 {
		bi.Prev, _ = w.db.GetBlockHash(bi.Height - 1)
	}
	if bi.Next == "" && bi.Height != bestheight {
		bi.Next, _ = w.db.GetBlockHash(bi.Height + 1)
	}
	txs = txs[:txi]
	bi.Txids = nil
	glog.Info("GetBlock ", bid, ", page ", page, " finished in ", time.Since(start))
	return &Block{
		Paging: pg,
		BlockInfo: BlockInfo{
			Hash:          bi.Hash,
			Prev:          bi.Prev,
			Next:          bi.Next,
			Height:        bi.Height,
			Confirmations: bi.Confirmations,
			Size:          bi.Size,
			Time:          bi.Time,
			Bits:          bi.Bits,
			Difficulty:    string(bi.Difficulty),
			MerkleRoot:    bi.MerkleRoot,
			Nonce:         string(bi.Nonce),
			Txids:         bi.Txids,
			Version:       bi.Version,
            MoneySupply:   bi.MoneySupply,
            ZerocoinSupply: bi.ZerocoinSupply,
		},
		TxCount:      txCount,
		Transactions: txs,
	}, nil
}

// ComputeFeeStats computes fee distribution in defined blocks and logs them to log
func (w *Worker) ComputeFeeStats(blockFrom, blockTo int, stopCompute chan os.Signal) error {
	bestheight, _, err := w.db.GetBestBlock()
	if err != nil {
		return errors.Annotatef(err, "GetBestBlock")
	}
	for block := blockFrom; block <= blockTo; block++ {
		hash, err := w.db.GetBlockHash(uint32(block))
		if err != nil {
			return err
		}
		bi, err := w.chain.GetBlockInfo(hash)
		if err != nil {
			return err
		}
		// process only blocks with enough transactions
		if len(bi.Txids) > 20 {
			dbi := &db.BlockInfo{
				Hash:   bi.Hash,
				Height: bi.Height,
				Time:   bi.Time,
			}
			txids := bi.Txids
			if w.chainType == bchain.ChainBitcoinType {
				// skip the coinbase transaction
				txids = txids[1:]
			}
			fees := make([]int64, len(txids))
			sum := int64(0)
			for i, txid := range txids {
				select {
				case <-stopCompute:
					glog.Info("ComputeFeeStats interrupted at height ", block)
					return db.ErrOperationInterrupted
				default:
					tx, err := w.txFromTxid(txid, bestheight, AccountDetailsTxHistoryLight, dbi)
					if err != nil {
						return err
					}
					fee := tx.FeesSat.AsInt64()
					fees[i] = fee
					sum += fee
				}
			}
			sort.Slice(fees, func(i, j int) bool { return fees[i] < fees[j] })
			step := float64(len(fees)) / 10
			percentils := ""
			for i := float64(0); i < float64(len(fees)+1); i += step {
				ii := int(math.Round(i))
				if ii >= len(fees) {
					ii = len(fees) - 1
				}
				percentils += "," + strconv.FormatInt(fees[ii], 10)
			}
			glog.Info(block, ",", time.Unix(bi.Time, 0).Format(time.RFC3339), ",", len(bi.Txids), ",", sum, ",", float64(sum)/float64(len(bi.Txids)), percentils)
		}
	}
	return nil
}

// GetSystemInfo returns information about system
func (w *Worker) GetSystemInfo(internal bool) (*SystemInfo, error) {
	start := time.Now()
	ci, err := w.chain.GetChainInfo()
	if err != nil {
		return nil, errors.Annotatef(err, "GetChainInfo")
	}
	vi := common.GetVersionInfo()
	inSync, bestHeight, lastBlockTime := w.is.GetSyncState()
	inSyncMempool, lastMempoolTime, mempoolSize := w.is.GetMempoolSyncState()
	var columnStats []common.InternalStateColumn
	var internalDBSize int64
	if internal {
		columnStats = w.is.GetAllDBColumnStats()
		internalDBSize = w.is.DBSizeTotal()
	}
	blockbookInfo := &BlockbookInfo{
		Coin:              w.is.Coin,
		Host:              w.is.Host,
		Version:           vi.Version,
		GitCommit:         vi.GitCommit,
		BuildTime:         vi.BuildTime,
		SyncMode:          w.is.SyncMode,
		InitialSync:       w.is.InitialSync,
		InSync:            inSync,
		BestHeight:        bestHeight,
		LastBlockTime:     lastBlockTime,
		InSyncMempool:     inSyncMempool,
		LastMempoolTime:   lastMempoolTime,
		MempoolSize:       mempoolSize,
		Decimals:          w.chainParser.AmountDecimals(),
		DbSize:            w.db.DatabaseSizeOnDisk(),
		DbSizeFromColumns: internalDBSize,
		DbColumns:         columnStats,
		About:             Text.BlockbookAbout,
	}
	backendInfo := &BackendInfo{
		Bestblockhash:   ci.Bestblockhash,
		Blocks:          ci.Blocks,
		Chain:           ci.Chain,
		Difficulty:      ci.Difficulty,
		Headers:         ci.Headers,
		ProtocolVersion: ci.ProtocolVersion,
		SizeOnDisk:      ci.SizeOnDisk,
		Subversion:      ci.Subversion,
		Timeoffset:      ci.Timeoffset,
		Version:         ci.Version,
		Warnings:        ci.Warnings,
        MoneySupply:     ci.MoneySupply,
        ZerocoinSupply:  ci.ZerocoinSupply,
        MasternodeCount: ci.MasternodeCount,
        NextSuperBlock:  ci.NextSuperBlock,
	}
	glog.Info("GetSystemInfo finished in ", time.Since(start))
	return &SystemInfo{blockbookInfo, backendInfo}, nil
}

// GetMempool returns a page of mempool txids
func (w *Worker) GetMempool(page int, itemsOnPage int) (*MempoolTxids, error) {
	page--
	if page < 0 {
		page = 0
	}
	entries := w.mempool.GetAllEntries()
	pg, from, to, _ := computePaging(len(entries), page, itemsOnPage)
	r := &MempoolTxids{
		Paging:      pg,
		MempoolSize: len(entries),
	}
	r.Mempool = make([]MempoolTxid, to-from)
	for i := from; i < to; i++ {
		entry := &entries[i]
		r.Mempool[i-from] = MempoolTxid{
			Txid: entry.Txid,
			Time: int64(entry.Time),
		}
	}
	return r, nil
}
