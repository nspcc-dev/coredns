package contract

import (
	"context"
	"errors"
	"fmt"
	"strings"

	nns "github.com/nspcc-dev/neo-go/examples/nft-nd-nns"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

type Contract struct {
	client       *client.Client
	contractHash util.Uint160
	nnsDomain    string
}

type Params struct {
	Endpoint     string
	ContractHash util.Uint160
	Domain       string
}

type Record struct {
	Name string
	Type nns.RecordType
	Data string
}

const dot = "."

func NewContract(ctx context.Context, prm *Params) (*Contract, error) {
	cli, err := client.New(ctx, prm.Endpoint, client.Options{})
	if err != nil {
		return nil, err
	}
	if err = cli.Init(); err != nil {
		return nil, err
	}

	if prm.ContractHash.Equals(util.Uint160{}) {
		cs, err := cli.GetContractStateByID(1)
		if err != nil {
			return nil, fmt.Errorf("get contract by id 1: %w", err)
		}
		prm.ContractHash = cs.Hash
	} else {
		if _, err = cli.GetContractStateByHash(prm.ContractHash); err != nil {
			return nil, fmt.Errorf("get contract '%s': %w", prm.ContractHash.StringLE(), err)
		}
	}

	return &Contract{
		client:       cli,
		contractHash: prm.ContractHash,
		nnsDomain:    strings.Trim(prm.Domain, dot),
	}, nil
}

func (c Contract) Hash() util.Uint160 {
	return c.contractHash
}

func (c *Contract) Resolve(name string, nnsType nns.RecordType) ([]string, error) {
	res, err := c.client.InvokeFunction(c.contractHash, "resolve", []smartcontract.Parameter{
		{
			Type:  smartcontract.StringType,
			Value: name,
		},
		{
			Type:  smartcontract.IntegerType,
			Value: int64(nnsType),
		},
	}, nil)
	if err != nil {
		return nil, err
	}

	if err = getInvocationError(res); err != nil {
		return nil, err
	}

	return getArrString(res.Stack)
}

func (c *Contract) GetAllRecords(name string) ([]Record, error) {
	res, err := c.client.InvokeFunction(c.contractHash, "getAllRecords", []smartcontract.Parameter{
		{
			Type:  smartcontract.StringType,
			Value: name,
		},
	}, nil)
	if err != nil {
		return nil, err
	}

	if err = getInvocationError(res); err != nil {
		return nil, err
	}

	return getRecordsIterator(res.Stack)
}

func (c *Contract) GetRecords(name string, nnsType nns.RecordType) ([]string, error) {
	res, err := c.client.InvokeFunction(c.contractHash, "getRecords", []smartcontract.Parameter{
		{
			Type:  smartcontract.StringType,
			Value: name,
		},
		{
			Type:  smartcontract.IntegerType,
			Value: int64(nnsType),
		},
	}, nil)
	if err != nil {
		return nil, err
	}

	if err = getInvocationError(res); err != nil {
		return nil, err
	}

	return getArrString(res.Stack)
}

func (c Contract) PrepareName(name, dnsDomain string) string {
	name = strings.TrimSuffix(name, dot)
	if c.nnsDomain != "" {
		name = strings.TrimSuffix(strings.TrimSuffix(name, dnsDomain), dot)
		if name != "" {
			name += dot
		}
		name += c.nnsDomain
	}
	return name
}

func getInvocationError(result *result.Invoke) error {
	if result.State != "HALT" {
		return fmt.Errorf("invocation failed: %s", result.FaultException)
	}
	if len(result.Stack) == 0 {
		return errors.New("result stack is empty")
	}
	return nil
}

func getArrString(st []stackitem.Item) ([]string, error) {
	index := len(st) - 1 // top stack element is last in the array
	arr, err := st[index].Convert(stackitem.ArrayT)
	if err != nil {
		return nil, err
	}
	if _, ok := arr.(stackitem.Null); ok {
		return nil, nil
	}

	iterator, ok := arr.Value().([]stackitem.Item)
	if !ok {
		return nil, errors.New("bad conversion")
	}

	res := make([]string, len(iterator))
	for i, item := range iterator {
		bs, err := item.TryBytes()
		if err != nil {
			return nil, err
		}
		res[i] = string(bs)
	}

	return res, nil
}

func getRecordsIterator(st []stackitem.Item) ([]Record, error) {
	index := len(st) - 1 // top stack element is last in the array
	tmp, err := st[index].Convert(stackitem.InteropT)
	if err != nil {
		return nil, err
	}
	iterator, ok := tmp.Value().(result.Iterator)
	if !ok {
		return nil, errors.New("bad conversion")
	}

	res := make([]Record, len(iterator.Values))
	for i, item := range iterator.Values {
		structArr, ok := item.Value().([]stackitem.Item)
		if !ok {
			return nil, errors.New("bad conversion")
		}
		if len(structArr) != 4 {
			return nil, errors.New("invalid response struct")
		}

		nameBytes, err := structArr[0].TryBytes()
		if err != nil {
			return nil, err
		}
		integer, err := structArr[1].TryInteger()
		if err != nil {
			return nil, err
		}
		typeBytes := integer.Bytes()
		if len(typeBytes) != 1 {
			return nil, errors.New("invalid nns type")
		}

		dataBytes, err := structArr[2].TryBytes()
		if err != nil {
			return nil, err
		}

		res[i] = Record{
			Name: string(nameBytes),
			Type: nns.RecordType(typeBytes[0]),
			Data: string(dataBytes),
		}
	}

	return res, nil
}
