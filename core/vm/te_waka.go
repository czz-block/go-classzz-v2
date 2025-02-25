// Copyright 2016 The go-classzz-v2 Authors
// This file is part of the go-classzz-v2 library.
//
// The go-classzz-v2 library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-classzz-v2 library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-classzz-v2 library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/classzz/go-classzz-v2/crypto"
	"github.com/classzz/go-classzz-v2/rpc"
	"math/big"
	"math/rand"
	"strings"
	"time"

	"github.com/classzz/go-classzz-v2/accounts/abi"
	"github.com/classzz/go-classzz-v2/common"
	"github.com/classzz/go-classzz-v2/core/types"
	"github.com/classzz/go-classzz-v2/log"
)

const (
	// Entangle Transcation type
	ExpandedTxConvert_Czz uint8 = iota
	ExpandedTxConvert_ECzz
	ExpandedTxConvert_HCzz
	ExpandedTxConvert_BCzz
	ExpandedTxConvert_OCzz
)

var (
	baseUnit  = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	Int10     = new(big.Int).Exp(big.NewInt(10), big.NewInt(10), nil)
	fbaseUnit = new(big.Float).SetFloat64(float64(baseUnit.Int64()))
	mixImpawn = new(big.Int).Mul(big.NewInt(1000), baseUnit)
	Base      = new(big.Int).SetUint64(10000)

	// i.e. contractAddress = 0x0000000000000000000000000000746577616b61
	TeWaKaAddress = common.BytesToAddress([]byte("tewaka"))
	CoinPools     = map[uint8]common.Address{
		ExpandedTxConvert_ECzz: {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 101},
		ExpandedTxConvert_HCzz: {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 102},
		ExpandedTxConvert_BCzz: {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 103},
		ExpandedTxConvert_OCzz: {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 104},
	}

	ethPoolAddr  = "0xB55c0fF37E2bA3Fd36AA03881373495A563E723c"
	hecoPoolAddr = "0x486c75523eC8A6797d66eD7Bf41F5079DCfDE185"
	bscPoolAddr  = ""
	okexPoolAddr = ""

	burnTopics = "0xd9ea7526cdb50f406e2429329000efebfed52962417d0ec902ab8ba0c3bc5f71"
	mintTopics = "0x8fb5c7bffbb272c541556c455c74269997b816df24f56dd255c2391d92d4f1e9"
)

// TeWaKaGas defines all method gas
var TeWaKaGas = map[string]uint64{
	"mortgage": 360000,
	"update":   360000,
	"convert":  2400000,
	"confirm":  2400000,
	"casting":  2400000,
}

// Staking contract ABI
var AbiTeWaKa abi.ABI
var AbiCzzRouter abi.ABI

type StakeContract struct{}

func init() {
	AbiTeWaKa, _ = abi.JSON(strings.NewReader(TeWakaABI))
	AbiCzzRouter, _ = abi.JSON(strings.NewReader(CzzRouterABI))
}

// RunStaking execute staking contract
func RunStaking(evm *EVM, contract *Contract, input []byte) (ret []byte, err error) {

	method, err := AbiTeWaKa.MethodById(input)

	if err != nil {
		log.Error("No method found")
		return nil, ErrExecutionReverted
	}

	data := input[4:]

	switch method.Name {
	case "mortgage":
		ret, err = mortgage(evm, contract, data)
	case "update":
		ret, err = update(evm, contract, data)
	case "convert":
		ret, err = convert(evm, contract, data)
	case "confirm":
		ret, err = confirm(evm, contract, data)
	case "casting":
		ret, err = casting(evm, contract, data)
	default:
		log.Warn("Staking call fallback function")
		err = ErrStakingInvalidInput
	}

	if err != nil {
		log.Warn("Staking error code", "code", err)
		err = ErrExecutionReverted
	}

	return ret, err
}

// logN add event log to receipt with topics up to 4
func logN(evm *EVM, contract *Contract, topics []common.Hash, data []byte) ([]byte, error) {
	evm.StateDB.AddLog(&types.Log{
		Address: contract.Address(),
		Topics:  topics,
		Data:    data,
		// This is a non-consensus field, but assigned here because
		// core/state doesn't know the current block number.
		BlockNumber: evm.Context.BlockNumber.Uint64(),
	})
	return nil, nil
}
func GenesisLockedBalance(db StateDB, from, to common.Address, amount *big.Int) {
	db.SubBalance(from, amount)
	db.AddBalance(to, amount)
}

// mortgage
func mortgage(evm *EVM, contract *Contract, input []byte) (ret []byte, err error) {
	t0 := time.Now()
	args := struct {
		PubKey          []byte
		ToAddress       common.Address
		StakingAmount   *big.Int
		CoinBaseAddress []common.Address
	}{}
	method, _ := AbiTeWaKa.Methods["mortgage"]

	err = method.Inputs.UnpackAtomic(&args, input)
	if err != nil {
		log.Error("Unpack deposit pubkey error", "err", err)
		return nil, ErrStakingInvalidInput
	}

	from := contract.caller.Address()

	t1 := time.Now()

	tewaka := NewTeWakaImpl()
	err = tewaka.Load(evm.StateDB, TeWaKaAddress)
	if err != nil {
		log.Error("Staking load error", "error", err)
		return nil, err
	}

	t2 := time.Now()

	tewaka.Mortgage(from, args.ToAddress, args.PubKey, args.StakingAmount, args.CoinBaseAddress)

	t3 := time.Now()
	err = tewaka.Save(evm.StateDB, TeWaKaAddress)
	if err != nil {
		log.Error("Staking save state error", "error", err)
		return nil, err
	}

	if have, want := evm.StateDB.GetBalance(from), args.StakingAmount; have.Cmp(want) < 0 {
		return nil, fmt.Errorf("%w: address %v have %v want %v", errors.New("insufficient funds for gas * price + value"), from, have, want)
	}

	evm.StateDB.SubBalance(from, args.StakingAmount)
	evm.StateDB.AddBalance(args.ToAddress, args.StakingAmount)

	t4 := time.Now()
	event := AbiTeWaKa.Events["mortgage"]
	logData, err := event.Inputs.Pack(args.ToAddress, args.StakingAmount, args.CoinBaseAddress)
	if err != nil {
		log.Error("Pack staking log error", "error", err)
		return nil, err
	}
	topics := []common.Hash{
		event.ID,
		common.BytesToHash(from[:]),
	}
	logN(evm, contract, topics, logData)
	context := []interface{}{
		"number", evm.Context.BlockNumber.Uint64(), "address", from, "StakingAmount", args.StakingAmount,
		"input", common.PrettyDuration(t1.Sub(t0)), "load", common.PrettyDuration(t2.Sub(t1)),
		"insert", common.PrettyDuration(t3.Sub(t2)), "save", common.PrettyDuration(t4.Sub(t3)),
		"log", common.PrettyDuration(time.Since(t4)), "elapsed", common.PrettyDuration(time.Since(t0)),
	}
	log.Info("mortgage", context...)
	return nil, nil
}

// Update
func update(evm *EVM, contract *Contract, input []byte) (ret []byte, err error) {
	t0 := time.Now()
	args := struct {
		CoinBaseAddress []common.Address
	}{}

	method, _ := AbiTeWaKa.Methods["update"]
	err = method.Inputs.UnpackAtomic(&args, input)
	if err != nil {
		log.Error("Unpack deposit pubkey error", "err", err)
		return nil, ErrStakingInvalidInput
	}

	from := contract.caller.Address()
	t1 := time.Now()

	tewaka := NewTeWakaImpl()
	err = tewaka.Load(evm.StateDB, TeWaKaAddress)
	if err != nil {
		log.Error("Staking load error", "error", err)
		return nil, err
	}

	t2 := time.Now()
	tewaka.Update(from, args.CoinBaseAddress)

	t3 := time.Now()
	err = tewaka.Save(evm.StateDB, TeWaKaAddress)
	if err != nil {
		log.Error("Staking save state error", "error", err)
		return nil, err
	}

	t4 := time.Now()
	event := AbiTeWaKa.Events["update"]
	logData, err := event.Inputs.Pack(args.CoinBaseAddress)
	if err != nil {
		log.Error("Pack staking log error", "error", err)
		return nil, err
	}
	topics := []common.Hash{
		event.ID,
		common.BytesToHash(from[:]),
	}
	logN(evm, contract, topics, logData)
	context := []interface{}{
		"number", evm.Context.BlockNumber.Uint64(), "address", from, "CoinBaseAddress", args.CoinBaseAddress,
		"input", common.PrettyDuration(t1.Sub(t0)), "load", common.PrettyDuration(t2.Sub(t1)),
		"insert", common.PrettyDuration(t3.Sub(t2)), "save", common.PrettyDuration(t4.Sub(t3)),
		"log", common.PrettyDuration(time.Since(t4)), "elapsed", common.PrettyDuration(time.Since(t0)),
	}
	log.Info("update", context...)
	return nil, nil
}

// Convert
func convert(evm *EVM, contract *Contract, input []byte) (ret []byte, err error) {
	t0 := time.Now()
	args := struct {
		AssetType *big.Int
		TxHash    string
	}{}

	method, _ := AbiTeWaKa.Methods["convert"]
	err = method.Inputs.UnpackAtomic(&args, input)
	if err != nil {
		log.Error("Unpack convert pubkey error", "err", err)
		return nil, ErrStakingInvalidInput
	}

	TxHash := common.HexToHash(args.TxHash)
	from := contract.caller.Address()
	t1 := time.Now()

	tewaka := NewTeWakaImpl()
	err = tewaka.Load(evm.StateDB, TeWaKaAddress)
	if err != nil {
		log.Error("Staking load error", "error", err)
		return nil, err
	}

	var item *types.ConvertItem
	AssetType := uint8(args.AssetType.Uint64())

	if exit := tewaka.HasItem(&types.UsedItem{AssetType, TxHash}, evm.StateDB); exit {
		return nil, ErrTxhashAlreadyInput
	}

	switch AssetType {
	case ExpandedTxConvert_ECzz:
		client := evm.chainConfig.EthClient[rand.Intn(len(evm.chainConfig.EthClient))]
		if item, err = verifyConvertEthereumTypeTx("ETH", evm, client, AssetType, TxHash); err != nil {
			return nil, err
		}
	case ExpandedTxConvert_HCzz:
		client := evm.chainConfig.HecoClient[rand.Intn(len(evm.chainConfig.HecoClient))]
		if item, err = verifyConvertEthereumTypeTx("HECO", evm, client, AssetType, TxHash); err != nil {
			return nil, err
		}
	case ExpandedTxConvert_BCzz:
		client := evm.chainConfig.BscClient[rand.Intn(len(evm.chainConfig.BscClient))]
		if item, err = verifyConvertEthereumTypeTx("BSC", evm, client, AssetType, TxHash); err != nil {
			return nil, err
		}
	case ExpandedTxConvert_OCzz:
		client := evm.chainConfig.OkexClient[rand.Intn(len(evm.chainConfig.OkexClient))]
		if item, err = verifyConvertEthereumTypeTx("OKEX", evm, client, AssetType, TxHash); err != nil {
			return nil, err
		}
	}

	item.FeeAmount = big.NewInt(0).Div(item.Amount, big.NewInt(1000))
	item.ID = big.NewInt(rand.New(rand.NewSource(time.Now().Unix())).Int63())
	item.Committee = tewaka.GetCommittee()

	t2 := time.Now()

	if item.ConvertType != ExpandedTxConvert_Czz {
		evm.StateDB.SubBalance(CoinPools[item.AssetType], item.Amount)
		evm.StateDB.AddBalance(CoinPools[item.ConvertType], new(big.Int).Sub(item.Amount, item.FeeAmount))
		tewaka.Convert(item)
	} else {
		evm.StateDB.SubBalance(CoinPools[item.AssetType], item.Amount)
		toaddresspuk, err := crypto.UnmarshalPubkey(item.PubKey)
		if err != nil || toaddresspuk == nil {
			return nil, err
		}
		toaddress := crypto.PubkeyToAddress(*toaddresspuk)
		evm.StateDB.AddBalance(toaddress, new(big.Int).Sub(item.Amount, item.FeeAmount))
	}

	tewaka.SetItem(&types.UsedItem{AssetType, TxHash})

	t3 := time.Now()
	err = tewaka.Save(evm.StateDB, TeWaKaAddress)
	if err != nil {
		log.Error("Staking save state error", "error", err)
		return nil, err
	}

	t4 := time.Now()
	event := AbiTeWaKa.Events["convert"]
	logData, err := event.Inputs.Pack(item.ID, args.AssetType, big.NewInt(int64(item.ConvertType)), item.TxHash.String(), item.Path, item.RouterAddr, item.PubKey, item.Committee, item.Amount, item.FeeAmount, item.Extra)
	if err != nil {
		log.Error("Pack staking log error", "error", err)
		return nil, err
	}
	topics := []common.Hash{
		event.ID,
		common.BytesToHash(from[:]),
	}
	logN(evm, contract, topics, logData)
	context := []interface{}{
		"number", evm.Context.BlockNumber.Uint64(), "address", from, "Amount", item.Amount,
		"AssetType", args.AssetType, "ConvertType", item.ConvertType, "TxHash", args.TxHash,
		"input", common.PrettyDuration(t1.Sub(t0)), "load", common.PrettyDuration(t2.Sub(t1)),
		"insert", common.PrettyDuration(t3.Sub(t2)), "save", common.PrettyDuration(t4.Sub(t3)),
		"log", common.PrettyDuration(time.Since(t4)), "elapsed", common.PrettyDuration(time.Since(t0)),
	}
	log.Info("convert", context...)

	return nil, nil
}

// Confirm
func confirm(evm *EVM, contract *Contract, input []byte) (ret []byte, err error) {
	t0 := time.Now()
	args := struct {
		ConvertType *big.Int
		TxHash      string
	}{}

	method, _ := AbiTeWaKa.Methods["confirm"]
	err = method.Inputs.UnpackAtomic(&args, input)
	if err != nil {
		log.Error("Unpack convert pubkey error", "err", err)
		return nil, ErrStakingInvalidInput
	}

	TxHash := common.HexToHash(args.TxHash)
	from := contract.caller.Address()
	t1 := time.Now()

	tewaka := NewTeWakaImpl()
	err = tewaka.Load(evm.StateDB, TeWaKaAddress)
	if err != nil {
		log.Error("Staking load error", "error", err)
		return nil, err
	}
	var item *types.ConvertItem
	ConvertType := uint8(args.ConvertType.Uint64())

	if exit := tewaka.HasItem(&types.UsedItem{ConvertType, TxHash}, evm.StateDB); exit {
		return nil, ErrTxhashAlreadyInput
	}

	switch ConvertType {
	case ExpandedTxConvert_ECzz:
		client := evm.chainConfig.EthClient[rand.Intn(len(evm.chainConfig.EthClient))]
		if item, err = verifyConfirmEthereumTypeTx("ETH", client, tewaka, ConvertType, TxHash); err != nil {
			return nil, err
		}
	case ExpandedTxConvert_HCzz:
		client := evm.chainConfig.HecoClient[rand.Intn(len(evm.chainConfig.HecoClient))]
		if item, err = verifyConfirmEthereumTypeTx("HECO", client, tewaka, ConvertType, TxHash); err != nil {
			return nil, err
		}
	case ExpandedTxConvert_BCzz:
		client := evm.chainConfig.BscClient[rand.Intn(len(evm.chainConfig.BscClient))]
		if item, err = verifyConfirmEthereumTypeTx("BSC", client, tewaka, ConvertType, TxHash); err != nil {
			return nil, err
		}
	case ExpandedTxConvert_OCzz:
		client := evm.chainConfig.OkexClient[rand.Intn(len(evm.chainConfig.OkexClient))]
		if item, err = verifyConfirmEthereumTypeTx("OKEX", client, tewaka, ConvertType, TxHash); err != nil {
			return nil, err
		}
	}

	t2 := time.Now()

	tewaka.Confirm(item)
	tewaka.SetItem(&types.UsedItem{ConvertType, TxHash})

	t3 := time.Now()
	err = tewaka.Save(evm.StateDB, TeWaKaAddress)
	if err != nil {
		log.Error("Staking save state error", "error", err)
		return nil, err
	}

	t4 := time.Now()
	event := AbiTeWaKa.Events["confirm"]
	logData, err := event.Inputs.Pack(item.ID, big.NewInt(int64(item.AssetType)), args.ConvertType, item.TxHash.String(), item.Path, item.PubKey, item.Committee, item.Amount, item.FeeAmount, item.Extra)
	if err != nil {
		log.Error("Pack staking log error", "error", err)
		return nil, err
	}
	topics := []common.Hash{
		event.ID,
		common.BytesToHash(from[:]),
	}
	logN(evm, contract, topics, logData)
	context := []interface{}{
		"number", evm.Context.BlockNumber.Uint64(), "address", from, "Amount", item.Amount,
		"AssetType", item.AssetType, "ConvertType", args.ConvertType, "TxHash", args.TxHash,
		"input", common.PrettyDuration(t1.Sub(t0)), "load", common.PrettyDuration(t2.Sub(t1)),
		"insert", common.PrettyDuration(t3.Sub(t2)), "save", common.PrettyDuration(t4.Sub(t3)),
		"log", common.PrettyDuration(time.Since(t4)), "elapsed", common.PrettyDuration(time.Since(t0)),
	}
	log.Info("convert", context...)

	return nil, nil
}

// Casting
func casting(evm *EVM, contract *Contract, input []byte) (ret []byte, err error) {
	t0 := time.Now()
	args := struct {
		ConvertType *big.Int
		Amount      *big.Int
		Path        []common.Address
	}{}

	method, _ := AbiTeWaKa.Methods["casting"]
	err = method.Inputs.UnpackAtomic(&args, input)
	if err != nil {
		log.Error("Unpack convert pubkey error", "err", err)
		return nil, ErrStakingInvalidInput
	}

	from := contract.caller.Address()
	t1 := time.Now()

	tewaka := NewTeWakaImpl()
	err = tewaka.Load(evm.StateDB, TeWaKaAddress)
	if err != nil {
		log.Error("Staking load error", "error", err)
		return nil, err
	}
	ConvertType := uint8(args.ConvertType.Uint64())

	item := &types.ConvertItem{
		ConvertType: ConvertType,
		Path:        args.Path,
		Amount:      args.Amount,
	}

	item.FeeAmount = big.NewInt(0).Div(item.Amount, big.NewInt(1000))
	item.ID = big.NewInt(rand.New(rand.NewSource(time.Now().Unix())).Int63())
	item.Committee = tewaka.GetCommittee()

	t2 := time.Now()
	tewaka.Convert(item)

	if have, want := evm.StateDB.GetBalance(from), args.Amount; have.Cmp(want) < 0 {
		return nil, fmt.Errorf("%w: address %v have %v want %v", errors.New("insufficient funds for gas * price + value"), from, have, want)
	}

	evm.StateDB.SubBalance(from, item.Amount)

	t3 := time.Now()
	err = tewaka.Save(evm.StateDB, TeWaKaAddress)
	if err != nil {
		log.Error("Staking save state error", "error", err)
		return nil, err
	}

	t4 := time.Now()
	event := AbiTeWaKa.Events["casting"]
	logData, err := event.Inputs.Pack(item.ID, args.ConvertType, item.TxHash.String(), item.Path, item.PubKey, item.Committee, item.Amount, item.FeeAmount, item.Extra)
	if err != nil {
		log.Error("Pack staking log error", "error", err)
		return nil, err
	}
	topics := []common.Hash{
		event.ID,
		common.BytesToHash(from[:]),
	}
	logN(evm, contract, topics, logData)
	context := []interface{}{
		"number", evm.Context.BlockNumber.Uint64(), "address", from, "Amount", item.Amount,
		"input", common.PrettyDuration(t1.Sub(t0)), "load", common.PrettyDuration(t2.Sub(t1)),
		"insert", common.PrettyDuration(t3.Sub(t2)), "save", common.PrettyDuration(t4.Sub(t3)),
		"log", common.PrettyDuration(time.Since(t4)), "elapsed", common.PrettyDuration(time.Since(t0)),
	}
	log.Info("convert", context...)

	return nil, nil
}

func verifyConvertEthereumTypeTx(netName string, evm *EVM, client *rpc.Client, AssetType uint8, TxHash common.Hash) (*types.ConvertItem, error) {

	var receipt *types.Receipt
	if err := client.Call(&receipt, "eth_getTransactionReceipt", TxHash); err != nil {
		return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) getTransactionReceipt [txid:%s] err: %s", netName, TxHash, err)
	}

	if receipt == nil {
		return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) [txid:%s] not find", netName, TxHash)
	}

	if receipt.Status != 1 {
		return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) [txid:%s] Status [%d]", netName, TxHash, receipt.Status)
	}

	if len(receipt.Logs) < 1 {
		return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s)  receipt Logs length is 0 ", netName)
	}

	var txLog *types.Log
	for _, log := range receipt.Logs {
		if log.Topics[0].String() == burnTopics {
			txLog = log
			break
		}
	}

	if txLog == nil {
		return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) txLog is nil ", netName)
	}

	logs := struct {
		Address    common.Address
		Amount     *big.Int
		Ntype      *big.Int
		ToPath     []common.Address
		RouterAddr common.Address
		Extra      []byte
	}{}

	if err := AbiCzzRouter.UnpackIntoInterface(&logs, "BurnToken", txLog.Data); err != nil {
		return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s)  UnpackIntoInterface err (%s)", netName, err)
	}

	amountPool := evm.StateDB.GetBalance(CoinPools[AssetType])
	TxAmount := new(big.Int).Mul(logs.Amount, Int10)
	if TxAmount.Cmp(amountPool) > 0 {
		return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) tx amount [%d] > pool [%d]", netName, TxAmount.Uint64(), amountPool)
	}

	if _, ok := CoinPools[uint8(logs.Ntype.Uint64())]; !ok && uint8(logs.Ntype.Uint64()) != 0 {
		return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) ConvertType is [%d] CoinPools not find", netName, logs.Ntype.Uint64())
	}

	if AssetType == uint8(logs.Ntype.Uint64()) {
		return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) AssetType = ConvertType = [%d]", netName, logs.Ntype.Uint64())
	}

	var extTx *types.Transaction
	// Get the current block count.
	if err := client.Call(&extTx, "eth_getTransactionByHash", TxHash); err != nil {
		return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) getTransactionByHash [txid:%s] err: %s", netName, TxHash, err)
	}

	if AssetType == ExpandedTxConvert_ECzz {
		if !strings.Contains(ethPoolAddr, extTx.To().String()) {
			return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) ETh [ToAddress: %s] != [%s]", netName, extTx.To().String(), ethPoolAddr)
		}
	} else if AssetType == ExpandedTxConvert_HCzz {
		if !strings.Contains(hecoPoolAddr, extTx.To().String()) {
			return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) Heco [ToAddress: %s] != [%s]", netName, extTx.To().String(), ethPoolAddr)
		}
	} else if AssetType == ExpandedTxConvert_BCzz {
		if !strings.Contains(bscPoolAddr, extTx.To().String()) {
			return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) Bsc [ToAddress: %s] != [%s]", netName, extTx.To().String(), ethPoolAddr)
		}
	} else if AssetType == ExpandedTxConvert_OCzz {
		if !strings.Contains(bscPoolAddr, extTx.To().String()) {
			return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) Bsc [ToAddress: %s] != [%s]", netName, extTx.To().String(), ethPoolAddr)
		}
	}

	Vb, R, S := extTx.RawSignatureValues()
	var V byte

	var chainID *big.Int
	if isProtectedV(Vb) {
		chainID = deriveChainId(Vb)
		V = byte(Vb.Uint64() - 35 - 2*chainID.Uint64())
	} else {
		V = byte(Vb.Uint64() - 27)
	}

	if !crypto.ValidateSignatureValues(V, R, S, false) {
		return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) ValidateSignatureValues err", netName)
	}
	// encode the signature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, crypto.SignatureLength)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V
	a := types.NewEIP155Signer(chainID)
	pk, err := crypto.Ecrecover(a.Hash(extTx).Bytes(), sig)
	if err != nil {
		return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) Ecrecover err: %s", netName, err)
	}

	item := &types.ConvertItem{
		AssetType:   AssetType,
		ConvertType: uint8(logs.Ntype.Uint64()),
		TxHash:      TxHash,
		PubKey:      pk,
		Amount:      logs.Amount,
		Path:        logs.ToPath,
		RouterAddr:  logs.RouterAddr,
		Extra:       logs.Extra,
	}

	return item, nil
}

func verifyConfirmEthereumTypeTx(netName string, client *rpc.Client, tewaka *TeWakaImpl, ConvertType uint8, TxHash common.Hash) (*types.ConvertItem, error) {

	var receipt *types.Receipt
	if err := client.Call(&receipt, "eth_getTransactionReceipt", TxHash); err != nil {
		return nil, fmt.Errorf("verifyConfirmEthereumTypeTx (%s) getTransactionReceipt [txid:%s] err: %s", netName, TxHash, err)
	}

	if receipt == nil {
		return nil, fmt.Errorf("verifyConfirmEthereumTypeTx (%s) [txid:%s] not find", netName, TxHash)
	}

	if receipt.Status != 1 {
		return nil, fmt.Errorf("verifyConfirmEthereumTypeTx (%s) [txid:%s] Status [%d]", netName, TxHash, receipt.Status)
	}

	if len(receipt.Logs) < 1 {
		return nil, fmt.Errorf("verifyConfirmEthereumTypeTx (%s)  receipt Logs length is 0 ", netName)
	}

	var txLog *types.Log
	for _, log := range receipt.Logs {
		if log.Topics[0].String() == mintTopics {
			txLog = log
			break
		}
	}

	if txLog == nil {
		return nil, fmt.Errorf("verifyConfirmEthereumTypeTx (%s) txLog is nil ", netName)
	}

	logs := struct {
		To       common.Address
		Amount   *big.Int
		Mid      *big.Int
		AmountIn *big.Int
	}{}

	if err := AbiCzzRouter.UnpackIntoInterface(&logs, "MintToken", txLog.Data); err != nil {
		return nil, fmt.Errorf("verifyConfirmEthereumTypeTx (%s)  UnpackIntoInterface err (%s)", netName, err)
	}
	logs.To = common.BytesToAddress(txLog.Topics[1][:])

	var item *types.ConvertItem
	for _, v := range tewaka.ConvertItems {
		if v.ID.Cmp(logs.Mid) == 0 {
			item = v
			break
		}
	}

	if item == nil {
		return nil, fmt.Errorf("verifyConfirmEthereumTypeTx (%s) ConvertItems [id:%d] is null", netName, logs.Mid.Uint64())
	}

	if item.ConvertType != ConvertType {
		return nil, fmt.Errorf("verifyConfirmEthereumTypeTx (%s) ConvertType is [%d] not [%d] ", netName, ConvertType, item.ConvertType)
	}

	toaddresspuk, err := crypto.DecompressPubkey(item.PubKey)
	if err != nil || toaddresspuk == nil {
		toaddresspuk, err = crypto.UnmarshalPubkey(item.PubKey)
		if err != nil || toaddresspuk == nil {
			return nil, fmt.Errorf("verifyConfirmEthereumTypeTx (%s) toaddresspuk [puk:%s] is err: %s", netName, hex.EncodeToString(item.PubKey), err)
		}
	}

	toaddress := crypto.PubkeyToAddress(*toaddresspuk)
	if logs.To.String() != toaddress.String() {
		return nil, fmt.Errorf("verifyConfirmEthereumTypeTx (%s) [toaddress : %s] not [toaddress2 : %s]", netName, logs.To.String(), toaddress.String())
	}

	amount2 := big.NewInt(0).Sub(item.Amount, item.FeeAmount)
	if logs.AmountIn.Cmp(amount2) != 0 {
		return nil, fmt.Errorf("verifyConfirmEthereumTypeTx (%s) amount %d not %d", netName, logs.Amount, amount2)
	}

	var extTx *types.Transaction
	// Get the current block count.
	if err := client.Call(&extTx, "eth_getTransactionByHash", TxHash); err != nil {
		return nil, err
	}

	if extTx == nil {
		return nil, fmt.Errorf("verifyConfirmEthereumTypeTx (%s) txjson is nil [txid:%s]", netName, TxHash)
	}

	// toaddress
	if ConvertType == ExpandedTxConvert_ECzz {
		if !strings.Contains(ethPoolAddr, extTx.To().String()) {
			return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) ETh [ToAddress: %s] != [%s]", netName, extTx.To().String(), ethPoolAddr)
		}
	} else if ConvertType == ExpandedTxConvert_HCzz {
		if !strings.Contains(hecoPoolAddr, extTx.To().String()) {
			return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) Heco [ToAddress: %s] != [%s]", netName, extTx.To().String(), hecoPoolAddr)
		}
	} else if ConvertType == ExpandedTxConvert_BCzz {
		if !strings.Contains(bscPoolAddr, extTx.To().String()) {
			return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) Bsc [ToAddress: %s] != [%s]", netName, extTx.To().String(), bscPoolAddr)
		}
	} else if ConvertType == ExpandedTxConvert_OCzz {
		if !strings.Contains(okexPoolAddr, extTx.To().String()) {
			return nil, fmt.Errorf("verifyConvertEthereumTypeTx (%s) Okex [ToAddress: %s] != [%s]", netName, extTx.To().String(), okexPoolAddr)
		}
	}

	return item, nil
}

func isProtectedV(V *big.Int) bool {
	if V.BitLen() <= 8 {
		v := V.Uint64()
		return v != 27 && v != 28
	}
	// anything not 27 or 28 is considered protected
	return true
}

// deriveChainId derives the chain id from the given v parameter
func deriveChainId(v *big.Int) *big.Int {
	if v.BitLen() <= 64 {
		v := v.Uint64()
		if v == 27 || v == 28 {
			return new(big.Int)
		}
		return new(big.Int).SetUint64((v - 35) / 2)
	}
	v = new(big.Int).Sub(v, big.NewInt(35))
	return v.Div(v, big.NewInt(2))
}

const TeWakaABI = `
[
  {
    "name": "mortgage",
    "inputs": [
        {
        "type": "address",
        "name": "toAddress"
      },
      {
        "type": "uint256",
        "unit": "wei",
        "name": "stakingAmount"
      },
      {
        "type": "address[]",
        "name": "coinBaseAddress"
      }
    ],
    "anonymous": false,
    "type": "event"
  },
  {
    "name": "mortgage",
    "outputs": [],
    "inputs": [
      {
        "type": "address",
        "name": "toAddress"
      },
      {
        "type": "uint256",
        "unit": "wei",
        "name": "stakingAmount"
      },
      {
        "type": "address[]",
        "name": "coinBaseAddress"
      }
    ],
    "constant": false,
    "payable": false,
    "type": "function"
  }, {
    "name": "update",
    "inputs": [
      {
        "type": "address[]",
        "name": "coinBaseAddress"
      }
    ],
    "anonymous": false,
    "type": "event"
  },
  {
    "name": "update",
    "outputs": [],
    "inputs": [
      {
        "type": "address[]",
        "name": "coinBaseAddress"
      }
    ],
    "constant": false,
    "payable": false,
    "type": "function"
  }, 
{
    "name": "convert",
    "inputs": [
       {
        "type": "uint256",
        "name": "ID"
      },{
        "type": "uint256",
        "name": "AssetType"
      },{
        "type": "uint256",
        "name": "ConvertType"
      },{
        "type": "string",
        "name": "TxHash"
      },{
        "type": "address[]",
        "name": "Path"
      },{
        "type": "address",
        "name": "RouterAddr"
      },{
        "type": "bytes",
        "name": "PubKey"
      },{
        "type": "address",
        "name": "Committee"
      },{
        "type": "uint256",
        "name": "Amount"
      },{
        "type": "uint256",
        "name": "FeeAmount"
      },{
        "type": "bytes",
        "name": "Extra"
      }
    ],
    "anonymous": false,
    "type": "event"
  },
  {
    "name": "convert",
    "outputs": [],
    "inputs": [
      {
        "type": "uint256",
        "name": "AssetType"
      },{
        "type": "string",
        "name": "TxHash"
      }
    ],
    "constant": false,
    "payable": false,
    "type": "function"
  },
{
    "name": "confirm",
    "inputs": [
     {
        "type": "uint256",
        "name": "ID"
      },{
        "type": "uint256",
        "name": "AssetType"
      },{
        "type": "uint256",
        "name": "ConvertType"
      },{
        "type": "string",
        "name": "TxHash"
      },{
        "type": "address[]",
        "name": "Path"
      },{
        "type": "bytes",
        "name": "PubKey"
      },{
        "type": "address",
        "name": "Committee"
      },{
        "type": "uint256",
        "name": "Amount"
      },{
        "type": "uint256",
        "name": "FeeAmount"
      },{
        "type": "bytes",
        "name": "Extra"
      }
    ],
    "anonymous": false,
    "type": "event"
  },
  {
    "name": "confirm",
    "outputs": [],
    "inputs": [
      {
        "type": "uint256",
        "name": "ConvertType"
      },{
        "type": "string",
        "name": "TxHash"
      }
    ],
    "constant": false,
    "payable": false,
    "type": "function"
  },
{
    "name": "casting",
    "inputs": [
  {
        "type": "uint256",
        "name": "ID"
      },{
        "type": "uint256",
        "name": "ConvertType"
      },{
        "type": "string",
        "name": "TxHash"
      },{
        "type": "address[]",
        "name": "Path"
      },{
        "type": "bytes",
        "name": "PubKey"
      },{
        "type": "address",
        "name": "Committee"
      },{
        "type": "uint256",
        "name": "Amount"
      },{
        "type": "uint256",
        "name": "FeeAmount"
      },{
        "type": "bytes",
        "name": "Extra"
      }
    ],
    "anonymous": false,
    "type": "event"
  },
  {
    "name": "casting",
    "outputs": [],
    "inputs": [
       {
        "type": "uint256",
        "name": "AssetType"
      },{
        "type": "uint256",
        "name": "Amount"
      },{
        "type": "address[]",
        "name": "Path"
      }
    ],
    "constant": false,
    "payable": false,
    "type": "function"
  }
]
`

const CzzRouterABI = `
[
	{
		"anonymous": false,
		"inputs": [
			{
				"indexed": true,
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"indexed": false,
				"internalType": "uint256",
				"name": "amount",
				"type": "uint256"
			},
			{
				"indexed": false,
				"internalType": "uint256",
				"name": "ntype",
				"type": "uint256"
			},
			{
				"indexed": false,
				"internalType": "address[]",
				"name": "toPath",
				"type": "address[]"
			},
			{
				"indexed": false,
				"internalType": "address",
				"name": "RouterAddr",
				"type": "address"
			},
			{
				"indexed": false,
				"internalType": "bytes",
				"name": "Extra",
				"type": "bytes"
			}
		],
		"name": "BurnToken",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{
				"indexed": true,
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"indexed": false,
				"internalType": "uint256",
				"name": "amount",
				"type": "uint256"
			},
			{
				"indexed": false,
				"internalType": "uint256",
				"name": "mid",
				"type": "uint256"
			},
			{
				"indexed": false,
				"internalType": "uint256",
				"name": "amountIn",
				"type": "uint256"
			}
		],
		"name": "MintToken",
		"type": "event"
	}
]
`
