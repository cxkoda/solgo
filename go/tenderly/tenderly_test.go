package tenderly

import (
	"bytes"
	"io"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/go-cmp/cmp"
)

func TestUnmarshalResponse(t *testing.T) {
	tests := []struct {
		name          string
		response      string
		unmarshalFunc func(body io.Reader) (any, error)
		want          any
	}{
		{
			name: "new fork",
			// The following response was obtained by calling the fork creation endpoint of Tenderly API manually using curl.
			// Some accounts were omitted.
			response:      `{"fork":{"id":"3869ed91-89eb-4628-8b9e-ea6b343380ba", "name":"NAME", "description":"DESC", "network_id":"1", "block_number":"17671140", "details":{"chain_config":{"chain_id":"1"}}, "accounts":{"0x5105CD53e1C87F8730104bBcD8328963B4f7C88A":"0x2b7622aacfa8a624067b9d8e16dbed6a3bd7e43cb47be262724e15550ce93ec3", "0x6973882056Af2Fc145868f5eE79469A99d931921":"0x86d8f2aea4e680a55582f883242ee8f9970ed74caab9b57cfbece9a0de997ead"}, "json_rpc_url":"https://rpc.tenderly.co/fork/3869ed91-89eb-4628-8b9e-ea6b343380ba"}}`,
			unmarshalFunc: func(body io.Reader) (any, error) { return unmarshalResponse[NewForkResponse](body) },
			want: &NewForkResponse{Fork: Fork{
				ID:          "3869ed91-89eb-4628-8b9e-ea6b343380ba",
				Name:        "NAME",
				Description: "DESC",
				NetworkID:   "1",
				BlockNumber: "17671140",
				Details: ForkDetails{
					ChainConfig: ForkDetailsChainConfig{
						ChainID: "1",
					},
				},
				Accounts: map[common.Address]string{
					common.HexToAddress("0x5105CD53e1C87F8730104bBcD8328963B4f7C88A"): "0x2b7622aacfa8a624067b9d8e16dbed6a3bd7e43cb47be262724e15550ce93ec3",
					common.HexToAddress("0x6973882056Af2Fc145868f5eE79469A99d931921"): "0x86d8f2aea4e680a55582f883242ee8f9970ed74caab9b57cfbece9a0de997ead",
				},
				NodeURL: "https://rpc.tenderly.co/fork/3869ed91-89eb-4628-8b9e-ea6b343380ba",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.unmarshalFunc(bytes.NewBuffer([]byte(tt.response)))
			if err != nil {
				t.Errorf("unmarshalResponse(%q) err = %v; want nil err", tt.response, err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("unmarshalResponse(%q) diff(+got -want)\n%s", tt.response, diff)
			}
		})
	}
}
