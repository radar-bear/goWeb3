package goWeb3

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/radar-bear/goWeb3/helper"
	"github.com/sirupsen/logrus"
	"math/big"
	"os"
	"strings"
)

// ============= Web3 =============

type Web3 struct {
	Rpc           *helper.EthRPC
	Accounts      []string
	privateKeyMap map[string]string // address -> privateKey
}

func NewWeb3(ethereumNodeUrl string) *Web3 {
	GetChainId()
	rpc := helper.NewEthRPC(ethereumNodeUrl)
	return &Web3{rpc, []string{}, map[string]string{}}
}

// ============= Account =============

func (w *Web3) AddAccount(privateKey string) (accountAddress string, err error) {
	pk, err := helper.NewPrivateKeyByHex(privateKey)
	if err != nil {
		return
	}
	accountAddress = strings.ToLower(helper.PubKey2Address(pk.PublicKey))
	w.Accounts = append(w.Accounts, accountAddress)
	w.privateKeyMap[accountAddress] = strings.ToLower(privateKey)
	return
}

func (w *Web3) BalanceOf(address string) (balance big.Int, err error) {
	return w.Rpc.EthGetBalance(address, "latest")
}

func (w *Web3) NonceOf(address string) (balance int, err error) {
	return w.Rpc.EthGetTransactionCount(address, "latest")
}

// todo: new block channel

// ============= Contract =============

type Contract struct {
	web3    *Web3
	abi     *abi.ABI
	address *common.Address
}

func (w *Web3) NewContract(abiStr string, address string) (contract *Contract, err error) {
	abi, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		return
	}

	commonAddress := common.HexToAddress(address)
	contract = &Contract{
		w, &abi, &commonAddress,
	}

	return
}

// ============= Interactions =============

type SendTxParams struct {
	FromAddress string
	GasLimit    *big.Int
	GasPrice    *big.Int
	Nonce       uint64
}

func (c *Contract) Call(functionName string, args ...interface{}) (resp string, err error) {
	if args != nil {
		return c.HistoryCall("latest", functionName, args)
	} else {
		return c.HistoryCall("latest", functionName)
	}
}

func (c *Contract) HistoryCall(blockNum string, functionName string, args ...interface{}) (resp string, err error) {
	var dataByte []byte
	if args != nil {
		dataByte, err = c.abi.Pack(functionName, args...)
	} else {
		dataByte = c.abi.Methods[functionName].ID()
	}
	if err != nil {
		return
	}

	return c.web3.Rpc.EthCall(helper.T{
		To:   c.address.String(),
		From: "0x0000000000000000000000000000000000000000",
		Data: fmt.Sprintf("0x%x", dataByte)},
		blockNum,
	)
}

func (c *Contract) Send(params *SendTxParams, value *big.Int, functionName string, args ...interface{}) (resp string, err error) {
	if _, ok := c.web3.privateKeyMap[strings.ToLower(params.FromAddress)]; !ok {
		err = errors.New("ACCOUNT_NOT_VALID")
		return
	}

	data, err := c.abi.Pack(functionName, args...)
	if err != nil {
		return
	}

	tx := types.NewTransaction(
		params.Nonce,
		*c.address,
		value,
		params.GasLimit.Uint64(),
		params.GasPrice,
		data,
	)
	rawData, _ := helper.SignTx(c.web3.privateKeyMap[strings.ToLower(params.FromAddress)], GetChainId(), tx)
	return c.web3.Rpc.EthSendRawTransaction(rawData)
}

func (w *Web3) TransferEth(params *SendTxParams, to string, value *big.Int) (resp string, err error) {
	tx := types.NewTransaction(
		params.Nonce,
		common.HexToAddress(to),
		value,
		params.GasLimit.Uint64(),
		params.GasPrice,
		[]byte{},
	)
	rawData, _ := helper.SignTx(w.privateKeyMap[strings.ToLower(params.FromAddress)], GetChainId(), tx)
	return w.Rpc.EthSendRawTransaction(rawData)
}

func (w *Web3) GetRecipt(txHash string) (receipt *helper.TransactionReceipt, err error) {
	return w.Rpc.EthGetTransactionReceipt(txHash)
}

// ============= Other Functions =============

func GetGasPriceGwei() (gasPriceInGwei int64) {
	resp, err := helper.Get("https://ethgasstation.info/json/ethgasAPI.json", "", helper.EmptyKeyPairList, helper.EmptyKeyPairList)
	if err != nil {
		return 30 // default 30gwei
	}
	var dataContainer struct {
		Fast    float64 `json:"fast"`
		Fastest float64 `json:"fastest"`
		SafeLow float64 `json:"safeLow"`
		Average float64 `json:"average"`
	}
	json.Unmarshal([]byte(resp), &dataContainer)
	gasPriceInGwei = int64(dataContainer.Fast / 10)
	return
}

func GetChainId() (chainId string) {
	network := os.Getenv("NETWORK")
	switch network {
	case "mainnet":
		return "1"
	case "kovan":
		return "42"
	default:
		logrus.Fatalf("%s network not support", network)
	}
	return "0"
}
