package nns

import (
	"errors"
	"fmt"

	nns "github.com/nspcc-dev/neo-go/examples/nft-nd-nns"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

type nnsRecord struct {
	Name string
	Type nns.RecordType
	Data string
}

func resolve(rpc *client.Client, hash util.Uint160, name string, nnsType nns.RecordType) ([]string, error) {
	res, err := rpc.InvokeFunction(hash, "resolve", []smartcontract.Parameter{
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

func getAllRecords(rpc *client.Client, hash util.Uint160, name string) ([]nnsRecord, error) {
	res, err := rpc.InvokeFunction(hash, "getAllRecords", []smartcontract.Parameter{
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

func getRecords(rpc *client.Client, hash util.Uint160, name string, nnsType nns.RecordType) ([]string, error) {
	res, err := rpc.InvokeFunction(hash, "getRecords", []smartcontract.Parameter{
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

func getRecordsIterator(st []stackitem.Item) ([]nnsRecord, error) {
	index := len(st) - 1 // top stack element is last in the array
	tmp, err := st[index].Convert(stackitem.InteropT)
	if err != nil {
		return nil, err
	}
	iterator, ok := tmp.Value().(result.Iterator)
	if !ok {
		return nil, errors.New("bad conversion")
	}

	res := make([]nnsRecord, len(iterator.Values))
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

		res[i] = nnsRecord{
			Name: string(nameBytes),
			Type: nns.RecordType(typeBytes[0]),
			Data: string(dataBytes),
		}
	}

	return res, nil
}
