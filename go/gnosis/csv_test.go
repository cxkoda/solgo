package gnosis

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gocarina/gocsv"
	"github.com/google/go-cmp/cmp"
	"github.com/shopspring/decimal"
)

func TestCSVConversion(t *testing.T) {
	tests := []struct {
		name     string
		transfer *GnosisCSVTransfer
		want     string
	}{
		{
			name:     "ETH",
			transfer: NewETHTransfer(decimal.NewFromFloat(0.02), common.HexToAddress("0x0000000000000000000000000000000000000001")),
			want: `token_type,token_address,receiver,amount,id
native,,0x0000000000000000000000000000000000000001,0.02,0
`,
		},
		{
			name:     "ERC20",
			transfer: NewERC20Transfer(common.HexToAddress("0x2000000000000000000000000000000000000000"), decimal.NewFromFloat(0.1337), common.HexToAddress("0x0000000000000000000000000000000000000002")),
			want: `token_type,token_address,receiver,amount,id
erc20,0x2000000000000000000000000000000000000000,0x0000000000000000000000000000000000000002,0.1337,0
`,
		},
		{
			name:     "ERC721",
			transfer: NewERC721Transfer(common.HexToAddress("0x3000000000000000000000000000000000000000"), 1337, common.HexToAddress("0x0000000000000000000000000000000000000003")),
			want: `token_type,token_address,receiver,amount,id
nft,0x3000000000000000000000000000000000000000,0x0000000000000000000000000000000000000003,0,1337
`,
		},
		{
			name:     "ERC1155",
			transfer: NewERC1155Transfer(common.HexToAddress("0x4000000000000000000000000000000000000000"), 42, 1337, common.HexToAddress("0x0000000000000000000000000000000000000004")),
			want: `token_type,token_address,receiver,amount,id
nft,0x4000000000000000000000000000000000000000,0x0000000000000000000000000000000000000004,1337,42
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txs := []*GnosisCSVTransfer{tt.transfer}
			got, err := gocsv.MarshalString(txs)
			if err != nil {
				t.Fatalf("gocsv.MarshalString(%+v): %v", txs, err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("gocsv.MarshalString(%+v) diff (+got -want): %s", txs, diff)
			}
		})
	}
}
