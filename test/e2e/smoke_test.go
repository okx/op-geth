//go:build !skip_smoke
// +build !skip_smoke

package e2e

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/test/constants"
	"github.com/ethereum/go-ethereum/test/operations"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

const (
	Gwei            = 1000000000
	blockAddress    = "0xdD2FD4581271e230360230F9337D5c0430Bf44C0"
	blockPrivateKey = "0xde9be858da4a475276426320d5e9262ecfc3ba460bfac56360bfa6c4c28b4ee0"

	testVerified                       = true
	tmpSenderPrivateKey                = "363ea277eec54278af051fb574931aec751258450a286edce9e1f64401f3b9c8"
	specificProjectSenderPrivateKey    = "100f4e42de757bdfa31122dfc5bc00f1afe508b7b3c214a96fa00cbf05d979cf"
	nonSpecificProjectSenderPrivateKay = "fc1c22e30c1d9e4f6449b3c44c2991dbc6a202d8895d18fefa224296c01949cd"
	erc20ABIJson                       = "[\n\t{\n\t\t\"inputs\": [],\n\t\t\"stateMutability\": \"nonpayable\",\n\t\t\"type\": \"constructor\"\n\t},\n\t{\n\t\t\"anonymous\": false,\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"indexed\": true,\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"owner\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"indexed\": true,\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"spender\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"indexed\": false,\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"value\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"Approval\",\n\t\t\"type\": \"event\"\n\t},\n\t{\n\t\t\"anonymous\": false,\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"indexed\": true,\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"from\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"indexed\": true,\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"to\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"indexed\": false,\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"value\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"Transfer\",\n\t\t\"type\": \"event\"\n\t},\n\t{\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"owner\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"spender\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"allowance\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"view\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"spender\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"amount\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"approve\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"bool\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"bool\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"nonpayable\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"account\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"balanceOf\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"view\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [],\n\t\t\"name\": \"decimals\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint8\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"uint8\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"view\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"spender\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"subtractedValue\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"decreaseAllowance\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"bool\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"bool\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"nonpayable\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"spender\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"addedValue\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"increaseAllowance\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"bool\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"bool\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"nonpayable\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [],\n\t\t\"name\": \"name\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"string\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"string\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"view\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [],\n\t\t\"name\": \"symbol\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"string\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"string\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"view\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [],\n\t\t\"name\": \"totalSupply\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"view\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"to\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"amount\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"transfer\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"bool\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"bool\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"nonpayable\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"from\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"to\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"amount\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"transferFrom\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"bool\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"bool\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"nonpayable\",\n\t\t\"type\": \"function\"\n\t}\n]"
	erc20BytecodeStr                   = "60806040523480156200001157600080fd5b506040518060400160405280600781526020017f4d79546f6b656e000000000000000000000000000000000000000000000000008152506040518060400160405280600381526020017f4d544b000000000000000000000000000000000000000000000000000000000081525081600390816200008f9190620004e4565b508060049081620000a19190620004e4565b505050620000e433620000b9620000ea60201b60201c565b600a620000c791906200075b565b6305f5e100620000d89190620007ac565b620000f360201b60201c565b620008e3565b60006012905090565b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff160362000165576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016200015c9062000858565b60405180910390fd5b62000179600083836200026060201b60201c565b80600260008282546200018d91906200087a565b92505081905550806000808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825401925050819055508173ffffffffffffffffffffffffffffffffffffffff16600073ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef83604051620002409190620008c6565b60405180910390a36200025c600083836200026560201b60201c565b5050565b505050565b505050565b600081519050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602260045260246000fd5b60006002820490506001821680620002ec57607f821691505b602082108103620003025762000301620002a4565b5b50919050565b60008190508160005260206000209050919050565b60006020601f8301049050919050565b600082821b905092915050565b6000600883026200036c7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff826200032d565b6200037886836200032d565b95508019841693508086168417925050509392505050565b6000819050919050565b6000819050919050565b6000620003c5620003bf620003b98462000390565b6200039a565b62000390565b9050919050565b6000819050919050565b620003e183620003a4565b620003f9620003f082620003cc565b8484546200033a565b825550505050565b600090565b6200041062000401565b6200041d818484620003d6565b505050565b5b8181101562000445576200043960008262000406565b60018101905062000423565b5050565b601f82111562000494576200045e8162000308565b62000469846200031d565b8101602085101562000479578190505b6200049162000488856200031d565b83018262000422565b50505b505050565b600082821c905092915050565b6000620004b96000198460080262000499565b1980831691505092915050565b6000620004d48383620004a6565b9150826002028217905092915050565b620004ef826200026a565b67ffffffffffffffff8111156200050b576200050a62000275565b5b620005178254620002d3565b6200052482828562000449565b600060209050601f8311600181146200055c576000841562000547578287015190505b620005538582620004c6565b865550620005c3565b601f1984166200056c8662000308565b60005b8281101562000596578489015182556001820191506020850194506020810190506200056f565b86831015620005b65784890151620005b2601f891682620004a6565b8355505b6001600288020188555050505b505050505050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b60008160011c9050919050565b6000808291508390505b60018511156200065957808604811115620006315762000630620005cb565b5b6001851615620006415780820291505b80810290506200065185620005fa565b945062000611565b94509492505050565b60008262000674576001905062000747565b8162000684576000905062000747565b81600181146200069d5760028114620006a857620006de565b600191505062000747565b60ff841115620006bd57620006bc620005cb565b5b8360020a915084821115620006d757620006d6620005cb565b5b5062000747565b5060208310610133831016604e8410600b8410161715620007185782820a905083811115620007125762000711620005cb565b5b62000747565b62000727848484600162000607565b92509050818404811115620007415762000740620005cb565b5b81810290505b9392505050565b600060ff82169050919050565b6000620007688262000390565b915062000775836200074e565b9250620007a47fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff848462000662565b905092915050565b6000620007b98262000390565b9150620007c68362000390565b9250828202620007d68162000390565b91508282048414831517620007f057620007ef620005cb565b5b5092915050565b600082825260208201905092915050565b7f45524332303a206d696e7420746f20746865207a65726f206164647265737300600082015250565b600062000840601f83620007f7565b91506200084d8262000808565b602082019050919050565b60006020820190508181036000830152620008738162000831565b9050919050565b6000620008878262000390565b9150620008948362000390565b9250828201905080821115620008af57620008ae620005cb565b5b92915050565b620008c08162000390565b82525050565b6000602082019050620008dd6000830184620008b5565b92915050565b61122f80620008f36000396000f3fe608060405234801561001057600080fd5b50600436106100a95760003560e01c80633950935111610071578063395093511461016857806370a082311461019857806395d89b41146101c8578063a457c2d7146101e6578063a9059cbb14610216578063dd62ed3e14610246576100a9565b806306fdde03146100ae578063095ea7b3146100cc57806318160ddd146100fc57806323b872dd1461011a578063313ce5671461014a575b600080fd5b6100b6610276565b6040516100c39190610b0c565b60405180910390f35b6100e660048036038101906100e19190610bc7565b610308565b6040516100f39190610c22565b60405180910390f35b61010461032b565b6040516101119190610c4c565b60405180910390f35b610134600480360381019061012f9190610c67565b610335565b6040516101419190610c22565b60405180910390f35b610152610364565b60405161015f9190610cd6565b60405180910390f35b610182600480360381019061017d9190610bc7565b61036d565b60405161018f9190610c22565b60405180910390f35b6101b260048036038101906101ad9190610cf1565b6103a4565b6040516101bf9190610c4c565b60405180910390f35b6101d06103ec565b6040516101dd9190610b0c565b60405180910390f35b61020060048036038101906101fb9190610bc7565b61047e565b60405161020d9190610c22565b60405180910390f35b610230600480360381019061022b9190610bc7565b6104f5565b60405161023d9190610c22565b60405180910390f35b610260600480360381019061025b9190610d1e565b610518565b60405161026d9190610c4c565b60405180910390f35b60606003805461028590610d8d565b80601f01602080910402602001604051908101604052809291908181526020018280546102b190610d8d565b80156102fe5780601f106102d3576101008083540402835291602001916102fe565b820191906000526020600020905b8154815290600101906020018083116102e157829003601f168201915b5050505050905090565b60008061031361059f565b90506103208185856105a7565b600191505092915050565b6000600254905090565b60008061034061059f565b905061034d858285610770565b6103588585856107fc565b60019150509392505050565b60006012905090565b60008061037861059f565b905061039981858561038a8589610518565b6103949190610ded565b6105a7565b600191505092915050565b60008060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b6060600480546103fb90610d8d565b80601f016020809104026020016040519081016040528092919081815260200182805461042790610d8d565b80156104745780601f1061044957610100808354040283529160200191610474565b820191906000526020600020905b81548152906001019060200180831161045757829003601f168201915b5050505050905090565b60008061048961059f565b905060006104978286610518565b9050838110156104dc576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016104d390610e93565b60405180910390fd5b6104e982868684036105a7565b60019250505092915050565b60008061050061059f565b905061050d8185856107fc565b600191505092915050565b6000600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054905092915050565b600033905090565b600073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff1603610616576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161060d90610f25565b60405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff1603610685576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161067c90610fb7565b60405180910390fd5b80600160008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508173ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925836040516107639190610c4c565b60405180910390a3505050565b600061077c8484610518565b90507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff81146107f657818110156107e8576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016107df90611023565b60405180910390fd5b6107f584848484036105a7565b5b50505050565b600073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff160361086b576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401610862906110b5565b60405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff16036108da576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016108d190611147565b60405180910390fd5b6108e5838383610a72565b60008060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490508181101561096b576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401610962906111d9565b60405180910390fd5b8181036000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550816000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825401925050819055508273ffffffffffffffffffffffffffffffffffffffff168473ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef84604051610a599190610c4c565b60405180910390a3610a6c848484610a77565b50505050565b505050565b505050565b600081519050919050565b600082825260208201905092915050565b60005b83811015610ab6578082015181840152602081019050610a9b565b60008484015250505050565b6000601f19601f8301169050919050565b6000610ade82610a7c565b610ae88185610a87565b9350610af8818560208601610a98565b610b0181610ac2565b840191505092915050565b60006020820190508181036000830152610b268184610ad3565b905092915050565b600080fd5b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b6000610b5e82610b33565b9050919050565b610b6e81610b53565b8114610b7957600080fd5b50565b600081359050610b8b81610b65565b92915050565b6000819050919050565b610ba481610b91565b8114610baf57600080fd5b50565b600081359050610bc181610b9b565b92915050565b60008060408385031215610bde57610bdd610b2e565b5b6000610bec85828601610b7c565b9250506020610bfd85828601610bb2565b9150509250929050565b60008115159050919050565b610c1c81610c07565b82525050565b6000602082019050610c376000830184610c13565b92915050565b610c4681610b91565b82525050565b6000602082019050610c616000830184610c3d565b92915050565b600080600060608486031215610c8057610c7f610b2e565b5b6000610c8e86828701610b7c565b9350506020610c9f86828701610b7c565b9250506040610cb086828701610bb2565b9150509250925092565b600060ff82169050919050565b610cd081610cba565b82525050565b6000602082019050610ceb6000830184610cc7565b92915050565b600060208284031215610d0757610d06610b2e565b5b6000610d1584828501610b7c565b91505092915050565b60008060408385031215610d3557610d34610b2e565b5b6000610d4385828601610b7c565b9250506020610d5485828601610b7c565b9150509250929050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602260045260246000fd5b60006002820490506001821680610da557607f821691505b602082108103610db857610db7610d5e565b5b50919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b6000610df882610b91565b9150610e0383610b91565b9250828201905080821115610e1b57610e1a610dbe565b5b92915050565b7f45524332303a2064656372656173656420616c6c6f77616e63652062656c6f7760008201527f207a65726f000000000000000000000000000000000000000000000000000000602082015250565b6000610e7d602583610a87565b9150610e8882610e21565b604082019050919050565b60006020820190508181036000830152610eac81610e70565b9050919050565b7f45524332303a20617070726f76652066726f6d20746865207a65726f2061646460008201527f7265737300000000000000000000000000000000000000000000000000000000602082015250565b6000610f0f602483610a87565b9150610f1a82610eb3565b604082019050919050565b60006020820190508181036000830152610f3e81610f02565b9050919050565b7f45524332303a20617070726f766520746f20746865207a65726f20616464726560008201527f7373000000000000000000000000000000000000000000000000000000000000602082015250565b6000610fa1602283610a87565b9150610fac82610f45565b604082019050919050565b60006020820190508181036000830152610fd081610f94565b9050919050565b7f45524332303a20696e73756666696369656e7420616c6c6f77616e6365000000600082015250565b600061100d601d83610a87565b915061101882610fd7565b602082019050919050565b6000602082019050818103600083015261103c81611000565b9050919050565b7f45524332303a207472616e736665722066726f6d20746865207a65726f20616460008201527f6472657373000000000000000000000000000000000000000000000000000000602082015250565b600061109f602583610a87565b91506110aa82611043565b604082019050919050565b600060208201905081810360008301526110ce81611092565b9050919050565b7f45524332303a207472616e7366657220746f20746865207a65726f206164647260008201527f6573730000000000000000000000000000000000000000000000000000000000602082015250565b6000611131602383610a87565b915061113c826110d5565b604082019050919050565b6000602082019050818103600083015261116081611124565b9050919050565b7f45524332303a207472616e7366657220616d6f756e742065786365656473206260008201527f616c616e63650000000000000000000000000000000000000000000000000000602082015250565b60006111c3602683610a87565b91506111ce82611167565b604082019050919050565b600060208201905081810360008301526111f2816111b6565b905091905056fea2646970667358221220a1e42afa780fa0b792c1b1544459f1223cd5f165dbd77fb15760adb1e937625e64736f6c63430008130033"
	erc20FreeGasAddressStr             = "0xAD1D01007a56EE0A4FFD0488fb58fC6500Cb1fbE"
)

func TestSendTx(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	TransToken(t, ctx, client, uint256.NewInt(params.GWei), operations.DefaultL2AdminAddress)

	from := common.HexToAddress(operations.DefaultL2AdminAddress)
	to := common.HexToAddress(operations.DefaultL2AdminAddress)
	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  from,
		To:    &to,
		Value: big.NewInt(10),
	})
	require.NoError(t, err)
	tx := types.NewTransaction(nonce, to, big.NewInt(10), gas, big.NewInt(100*params.GWei), nil)
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	err = operations.WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)
}

func TestChainID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	chainID, err := operations.EthChainID()
	require.NoError(t, err)
	require.Equal(t, chainID, operations.DefaultL2ChainID)
}

func TestEthTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	if !testVerified {
		return
	}

	ctx := context.Background()
	auth, err := operations.GetAuth(operations.DefaultL2AdminPrivateKey, operations.DefaultL2ChainID)
	require.NoError(t, err)
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)

	from := common.HexToAddress(operations.DefaultL2AdminAddress)
	to := common.HexToAddress(operations.DefaultL2NewAcc1Address)
	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	tx := types.NewTransaction(nonce, to, big.NewInt(0), 21000, big.NewInt(10*params.GWei), nil)
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)
	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	require.NoError(t, err)
	var txs []*types.Transaction
	txs = append(txs, signedTx)
	_, err = operations.ApplyL2Txs(ctx, txs, auth, client, operations.VerifiedConfirmationLevel)
	require.NoError(t, err)
}

func TestDebugTraceRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Wait for at least one block to be available
	var blockNumber uint64
	var err error
	for i := 0; i < 30; i++ {
		blockNumber, err = operations.GetBlockNumber()
		require.NoError(t, err)
		log.Info("Block number: %d, attempt: %v", blockNumber, i)
		if blockNumber > 3 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	require.Greater(t, blockNumber, uint64(0), "Block number should be greater than 0")

	// Get a block to trace
	blockNum, err := operations.GetBlockNumber()
	require.NoError(t, err)

	// Use the working RPC method instead of the broken GetBlockByNumber
	blockNumberHex := fmt.Sprintf("0x%x", blockNum)
	blockData, err := operations.EthGetBlockByNumber(blockNumberHex, true)
	require.NoError(t, err)
	require.NotNil(t, blockData, "Block data should not be nil")

	blockHash := common.Hash{}
	if blockMap, ok := blockData.(map[string]interface{}); ok {
		if hashStr, exists := blockMap["hash"].(string); exists && hashStr != "" {
			blockHash = common.HexToHash(hashStr)
		}
	}

	// Test debug_traceBlockByHash
	t.Run("DebugTraceBlockByHash", func(t *testing.T) {
		require.NotEqual(t, common.Hash{}, blockHash, "Block hash should not be empty")

		traceResult, err := operations.DebugTraceBlockByHash(blockHash)
		require.NoError(t, err)
		require.NotNil(t, traceResult, "Trace result should not be nil")

		log.Info("DebugTraceBlockByHash result type: %T", traceResult)
	})

	// Test debug_traceBlockByNumber
	t.Run("DebugTraceBlockByNumber", func(t *testing.T) {
		traceResult, err := operations.DebugTraceBlockByNumber(blockNum)
		require.NoError(t, err)
		require.NotNil(t, traceResult, "Trace result should not be nil")

		log.Info("DebugTraceBlockByNumber result type: %T", traceResult)
	})

	// Test debug_traceTransaction
	t.Run("DebugTraceTransaction", func(t *testing.T) {
		blockInfo, err := operations.EthGetBlockByHash(blockHash, true)
		require.NoError(t, err)
		var blockInfoMap map[string]interface{}
		var txsInterface interface{}
		var txs []interface{}
		var txMap map[string]interface{}
		var txHashStr string
		var exists bool
		var ok bool

		if blockInfoMap, ok = blockInfo.(map[string]interface{}); !ok {
			t.Error("Block info not in expected format")
		}
		if txsInterface, exists = blockInfoMap["transactions"]; !exists {
			t.Error("Transactions field not found in block data")
		}

		if txs, ok = txsInterface.([]interface{}); !ok || len(txs) == 0 {
			t.Error("No transactions found in block")
		}

		if txMap, ok = txs[0].(map[string]interface{}); !ok {
			t.Error("Transaction data not in expected format")
		}

		if txHashStr, exists = txMap["hash"].(string); !exists {
			t.Error("Transaction hash not found in transaction data")
		}

		txHash := common.HexToHash(txHashStr)
		require.NotEqual(t, common.Hash{}, txHash, "Transaction hash should not be empty")

		traceResult, err := operations.DebugTraceTransaction(txHash)
		require.NoError(t, err)
		require.NotNil(t, traceResult, "Trace result should not be nil")

		log.Info("DebugTraceTransaction result type: %T", traceResult)
	})
}

// TestEthereumBasicRPC tests basic Ethereum RPC methods
func TestEthereumBasicRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	_, _ = SetupTestEnvironment(t)

	// Default test address for tests that require an address
	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Test eth_chainId
	t.Run("EthChainID", func(t *testing.T) {
		chainID, err := operations.EthChainID()
		require.NoError(t, err)
		require.NotEqual(t, uint64(0), chainID, "Chain ID should not be zero")
		log.Info("EthChainID result: %d", chainID)
	})

	// Test eth_syncing
	t.Run("EthSyncing", func(t *testing.T) {
		syncing, err := operations.EthSyncing()
		require.NoError(t, err)
		log.Info("EthSyncing result: %t", syncing)
	})

	// Test eth_getBalance
	t.Run("EthGetBalance", func(t *testing.T) {
		balance, err := operations.EthGetBalance(testAddress, "latest")
		require.NoError(t, err)
		log.Info("EthGetBalance result for test address: %s", balance.String())
	})

	// Test eth_getCode
	t.Run("EthGetCode", func(t *testing.T) {
		code, err := operations.EthGetCode(testAddress, "latest")
		require.NoError(t, err)
		log.Info("EthGetCode result length: %d", len(code))
	})

	// Test eth_getTransactionCount
	t.Run("EthGetTransactionCount", func(t *testing.T) {
		txCount, err := operations.EthGetTransactionCount(testAddress, "latest")
		require.NoError(t, err)
		log.Info("EthGetTransactionCount result: %d", txCount)
	})

	// Test eth_blockNumber
	t.Run("EthBlockNumber", func(t *testing.T) {
		blockNumber, err := operations.EthBlockNumber()
		require.NoError(t, err)
		require.Greater(t, blockNumber, uint64(0), "Block number should be greater than 0")
		log.Info("EthBlockNumber result: %d", blockNumber)
	})

	// Test eth_gasPrice
	t.Run("EthGasPrice", func(t *testing.T) {
		gasPrice, err := operations.EthGasPrice()
		require.NoError(t, err)
		require.Greater(t, gasPrice.Cmp(big.NewInt(0)), 0, "Gas price should be greater than 0")
		log.Info("EthGasPrice result: %s", gasPrice.String())
	})

	// Test eth_getStorageAt
	t.Run("EthGetStorageAt", func(t *testing.T) {
		storage, err := operations.EthGetStorageAt(testAddress, "0x0", "latest")
		require.NoError(t, err)
		log.Info("EthGetStorageAt result: %s", storage)
	})
}

// TestEthereumBlockRPC tests Ethereum block-related RPC methods
func TestEthereumBlockRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	EnsureContractsDeployed(t)

	blockHash, blockNumber := SetupTestEnvironment(t)
	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	defer client.Close()

	toAddr := common.HexToAddress(operations.DefaultL2NewAcc1Address)

	// Test eth_getBlockByHash
	t.Run("EthGetBlockByHash", func(t *testing.T) {
		block, err := operations.EthGetBlockByHash(blockHash, true)
		require.NoError(t, err)
		require.NotNil(t, block, "Block should not be nil")
		log.Info("EthGetBlockByHash result type: %T", block)
	})

	// Test eth_getBlockByNumber
	t.Run("EthGetBlockByNumber", func(t *testing.T) {
		blockNumberHex := fmt.Sprintf("0x%x", blockNumber)
		block, err := operations.EthGetBlockByNumber(blockNumberHex, true)
		require.NoError(t, err)
		require.NotNil(t, block, "Block should not be nil")
		log.Info("EthGetBlockByNumber result type: %T", block)
	})

	// Test eth_getBlockTransactionCountByHash
	t.Run("EthGetBlockTransactionCountByHash", func(t *testing.T) {
		txCount, err := operations.EthGetBlockTransactionCountByHash(blockHash)
		require.NoError(t, err)
		log.Info("EthGetBlockTransactionCountByHash result: %d", txCount)
	})

	// Test eth_getBlockTransactionCountByNumber
	t.Run("EthGetBlockTransactionCountByNumber", func(t *testing.T) {
		// Use current block instead of pruned block #1
		currentBlockHex := fmt.Sprintf("0x%x", blockNumber)
		txCount, err := operations.EthGetBlockTransactionCountByNumber(currentBlockHex)
		require.NoError(t, err)
		log.Info("EthGetBlockTransactionCountByNumber result: %d", txCount)
	})

	// Test eth_getTransactionByBlockHashAndIndex
	t.Run("EthGetTransactionByBlockHashAndIndex", func(t *testing.T) {
		tx, err := operations.EthGetTransactionByBlockHashAndIndex(blockHash, "0x0")
		require.NoError(t, err)
		log.Info("EthGetTransactionByBlockHashAndIndex result type: %T", tx)
	})

	// Test eth_getTransactionByBlockNumberAndIndex
	t.Run("EthGetTransactionByBlockNumberAndIndex", func(t *testing.T) {
		// Use current block instead of pruned block #1
		currentBlockHex := fmt.Sprintf("0x%x", blockNumber)
		tx, err := operations.EthGetTransactionByBlockNumberAndIndex(currentBlockHex, "0x0")
		require.NoError(t, err)
		require.NotNil(t, tx, "Transaction should not be nil")
		log.Info("EthGetTransactionByBlockNumberAndIndex result type: %T", tx)
	})

	t.Run("EthGetBlockReceipts", func(t *testing.T) {
		numberOfTransactions := 10
		_, targetBlockNumber, targetBlockHash := transErc20TokenBatch(t, context.Background(), client, ERC20Addr, big.NewInt(Gwei), toAddr.String(), numberOfTransactions)

		// Test getting receipts by number
		receiptsByNumber, err := client.BlockReceipts(ctx, rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(targetBlockNumber)))
		require.NoError(t, err)
		require.NotNil(t, receiptsByNumber, "Transaction receipts by number should not be nil")
		for _, receipt := range receiptsByNumber {
			require.NoError(t, err)
			require.NotNil(t, receipt)
			log.Info(fmt.Sprintf("RealtimeGetBlockReceiptsByNumber result type: %T", receipt))
		}

		// Test getting receipts by hash
		receiptsByHash, err := client.BlockReceipts(ctx, rpc.BlockNumberOrHashWithHash(targetBlockHash, true))
		require.NoError(t, err)
		require.NotNil(t, receiptsByHash, "Transaction receipts by hash should not be nil")
		for _, receipt := range receiptsByHash {
			require.NoError(t, err)
			require.NotNil(t, receipt)
			log.Info(fmt.Sprintf("RealtimeGetBlockReceiptsByHash result type: %T", receipt))
		}
	})
}

// TestEthereumTransactionRPC tests Ethereum transaction-related RPC methods
func TestEthereumTransactionRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	EnsureContractsDeployed(t)
	fromAddr := common.HexToAddress(operations.DefaultL2AdminAddress)
	toAddr := common.HexToAddress(operations.DefaultL2NewAcc1Address)

	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	defer client.Close()

	txhash := TransToken(t, ctx, client, uint256.NewInt(params.GWei), operations.DefaultL2AdminAddress)
	fmt.Printf("TransToken txhash: %s\n", txhash)

	t.Run("EthEstimateGas", func(t *testing.T) {
		balance, err := client.BalanceAt(ctx, fromAddr, nil)
		require.NoError(t, err)
		require.Greater(t, balance.Cmp(big.NewInt(0)), 0, "From address should have balance")

		t.Run("SimpleTransfer", func(t *testing.T) {
			transferAmount := big.NewInt(1000000000000000)

			gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
				From:  fromAddr,
				To:    &toAddr,
				Value: transferAmount,
			})
			require.NoError(t, err)
			require.Equal(t, uint64(21000), gas, "Simple transfer should use exactly 21000 gas")

			fmt.Printf("EthEstimateGas SimpleTransfer result: %d gas\n", gas)
		})

		// Test 2: Contract call gas estimation
		t.Run("ContractCall", func(t *testing.T) {
			contractAABI, err := abi.JSON(strings.NewReader(constants.ContractAABIJson))
			require.NoError(t, err)
			calldata, err := contractAABI.Pack("triggerCall")
			require.NoError(t, err)

			gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
				From: fromAddr,
				To:   &ContractAAddr,
				Data: calldata,
			})
			require.NoError(t, err)
			require.Greater(t, gas, uint64(21000), "Contract call should use more than 21000 gas")

			fmt.Printf("EthEstimateGas ContractCall result: %d gas\n", gas)
		})

	})

	t.Run("EthCall", func(t *testing.T) {
		// Get balance before the call
		balanceBefore, err := client.BalanceAt(ctx, fromAddr, nil)
		require.NoError(t, err)
		require.Greater(t, balanceBefore.Cmp(big.NewInt(0)), 0, "From address should have balance")

		contractCABI, err := abi.JSON(strings.NewReader(constants.ContractCABIJson))
		require.NoError(t, err)
		calldata, err := contractCABI.Pack("getValue")
		require.NoError(t, err)

		result, err := client.CallContract(ctx, ethereum.CallMsg{
			From: fromAddr,
			To:   &ContractCAddr,
			Data: calldata,
		}, nil)
		require.NoError(t, err)
		require.NotNil(t, result, "Call result should not be nil")

		balanceAfter, err := client.BalanceAt(ctx, fromAddr, nil)
		require.NoError(t, err)
		require.Equal(t, balanceBefore.String(), balanceAfter.String(), "Balance should remain unchanged after eth_call")
	})

	t.Run("EthGetTransactionByHash", func(t *testing.T) {
		result, err := operations.EthGetTransactionByHash(common.HexToHash(txhash))
		require.NoError(t, err)

		txData, _ := result.(map[string]interface{})

		receipt, err := client.TransactionReceipt(ctx, common.HexToHash(txhash))
		require.NoError(t, err)
		require.NotNil(t, receipt, "Receipt should not be nil")

		txHashStr, exists := txData["hash"].(string)
		require.True(t, exists, "Transaction should have hash field")
		require.Equal(t, txhash, txHashStr, "Transaction hash should match")

		txIndexHex := txData["transactionIndex"].(string)
		txIndex, err := hexutil.DecodeUint64(txIndexHex)
		require.NoError(t, err, "Transaction index should be valid hex")
		require.Equal(t, uint64(receipt.TransactionIndex), txIndex, "Transaction index should match between transaction and receipt")

		fromAddr, exists := txData["from"].(string)
		require.True(t, exists, "Transaction should have from field")
		require.Equal(t, strings.ToLower(operations.DefaultL2AdminAddress), strings.ToLower(fromAddr), "From address should match")
	})

	t.Run("EthGetTransactionReceipt", func(t *testing.T) {
		// Get receipt using operations RPC method
		result, err := operations.EthGetTransactionReceipt(common.HexToHash(txhash))
		require.NoError(t, err)

		receiptData, _ := result.(map[string]interface{})

		fromAddrStr, exists := receiptData["from"].(string)
		require.True(t, exists, "Receipt should have from field")
		require.Equal(t, strings.ToLower(operations.DefaultL2AdminAddress), strings.ToLower(fromAddrStr), "from address should match sender")

		statusStr, exists := receiptData["status"].(string)
		require.True(t, exists, "Receipt should have status field")
		require.Equal(t, "0x1", statusStr, "status should be 0x1 for successful transaction")

		toAddrStr, exists := receiptData["to"].(string)
		require.True(t, exists, "Receipt should have to field")
		require.Equal(t, strings.ToLower(operations.DefaultL2AdminAddress), strings.ToLower(toAddrStr), "to address should match recipient")

		txHashStr, exists := receiptData["transactionHash"].(string)
		require.True(t, exists, "Receipt should have transactionHash field")
		require.Equal(t, txhash, txHashStr, "transactionHash should match the original transaction hash")
	})
}

// TestEthereumLogsRPC tests Ethereum logs-related RPC methods
func TestEthereumLogsRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	_, blockNumber := SetupTestEnvironment(t)

	// Test eth_getLogs
	t.Run("EthGetLogs", func(t *testing.T) {
		fromBlock := fmt.Sprintf("0x%x", blockNumber)
		toBlock := fmt.Sprintf("0x%x", blockNumber)
		address := common.HexToAddress("0x1234567890123456789012345678901234567890")

		logs, err := operations.EthGetLogs(fromBlock, toBlock, address)
		require.NoError(t, err)
		require.NotNil(t, logs, "Logs should not be nil")
		log.Info("EthGetLogs result type: %T", logs)
	})
}

// TestTxPoolRPC tests transaction pool related RPC methods
func TestTxPoolRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	_, _ = SetupTestEnvironment(t)

	// Test txpool_content - This might return a large object, so only log type
	t.Run("TxPoolContent", func(t *testing.T) {
		content, err := operations.TxPoolContent()
		require.NoError(t, err)
		log.Info("TxPoolContent result type: %T", content)
	})

	// Test txpool_status
	t.Run("TxPoolStatus", func(t *testing.T) {
		status, err := operations.TxPoolStatus()
		require.NoError(t, err)
		log.Info("TxPoolStatus result type: %T", status)
	})
}

// TestInnerTransactionRPC tests inner transaction related RPC methods
func TestInnerTx(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	defer client.Close()

	EnsureContractsDeployed(t)

	preexecPrivateKey, err := crypto.HexToECDSA(TmpSenderPrivateKey)
	require.NoError(t, err)
	preexecFrom := crypto.PubkeyToAddress(preexecPrivateKey.PublicKey)

	// Call ContractA's triggerCall function which will call ContractB's dummy function
	// triggerCall() function selector: 0xf18c388a
	triggerCallData := common.Hex2Bytes("f18c388a")

	signedContractATxHash, err := MakeContractCall(t, ctx, client, preexecPrivateKey, ContractAAddr, triggerCallData, 200000, nil)
	require.NoError(t, err)

	contractAReceipt, err := client.TransactionReceipt(ctx, signedContractATxHash)
	require.NoError(t, err)

	// Call ContractC's setValue function
	contractCSetValueData := common.Hex2Bytes("552410770000000000000000000000000000000000000000000000000000000000000123")
	signedContractCSetValueTxHash, err := MakeContractCall(t, ctx, client, preexecPrivateKey, ContractCAddr, contractCSetValueData, 200000, nil)
	require.NoError(t, err)
	fmt.Printf("signedContractCSetValueTxHash: %s\n", signedContractCSetValueTxHash.Hex())

	// Call ContractC's getValue function
	contractCGetValueData := common.Hex2Bytes("20965255")
	signedContractCGetValueTxHash, err := MakeContractCall(t, ctx, client, preexecPrivateKey, ContractCAddr, contractCGetValueData, 200000, nil)
	require.NoError(t, err)
	contractCGetValueReceipt, err := client.TransactionReceipt(ctx, signedContractCGetValueTxHash)
	require.NoError(t, err)

	t.Run("GetInternalTransactions", func(t *testing.T) {
		innerTxs, err := operations.EthGetInternalTransactions(signedContractCGetValueTxHash)
		require.NoError(t, err)
		require.NotNil(t, innerTxs, "Inner transactions result should not be nil")
		require.Len(t, innerTxs, 1, "Should have exactly 1 inner transaction for getValue call")

		innerTx := innerTxs[0]
		require.NotNil(t, innerTx, "innerTx should not be nil")

		expectedInnerTx := &types.InnerTx{
			Dept:          *big.NewInt(0),
			InternalIndex: *big.NewInt(0),
			CallType:      "",
			Name:          "",
			TraceAddress:  "",
			CodeAddress:   "",
			From:          preexecFrom.Hex(),
			To:            ContractCAddr.Hex(),
			Input:         "",
			Output:        "0x0000000000000000000000000000000000000000000000000000000000000123",
			IsError:       false,
			Error:         "",
			Value:         "",
			ValueWei:      "0",
			CallValueWei:  "0x0",
		}

		ValidateInnerTransactionMatch(t, innerTx, expectedInnerTx, "ContractC getValue inner transaction")
		require.Equal(t, contractCGetValueReceipt.GasUsed, innerTx.GasUsed, "GasUsed should be the same as the gasUsed in transaction receipt for getValue call")
		require.Equal(t, uint64(200000), innerTx.Gas, "Gas should be the same as the gas in transaction receipt for getValue call")
	})

	t.Run("GetInternalTransactions_WithDeepInnerTransactions", func(t *testing.T) {
		innerTxs, err := operations.EthGetInternalTransactions(signedContractATxHash)
		require.NoError(t, err)
		require.NotNil(t, innerTxs, "Inner transactions result should not be nil")
		require.Len(t, innerTxs, 2, "Should have exactly 2 inner transactions: EOA->ContractA and ContractA->ContractB")

		// Validate first inner transaction: EOA -> ContractA (triggerCall)
		expectedInnertx1 := &types.InnerTx{
			Dept:          *big.NewInt(0),
			InternalIndex: *big.NewInt(0),
			CallType:      "",
			Name:          "",
			TraceAddress:  "",
			CodeAddress:   "",
			From:          preexecFrom.Hex(),
			To:            ContractAAddr.Hex(),
			Input:         "",
			Output:        "",
			IsError:       false,
			Error:         "",
			Value:         "",
			ValueWei:      "0",
			CallValueWei:  "0x0",
		}
		ValidateInnerTransactionMatch(t, innerTxs[0], expectedInnertx1, "First inner transaction (EOA -> ContractA)")

		// gasUsed of first inner transaction should be the same as the gasUsed in transaction receipt
		require.Equal(t, contractAReceipt.GasUsed, innerTxs[0].GasUsed, "First inner transaction GasUsed should be the same as contractAReceipt.GasUsed")
		require.Equal(t, uint64(200000), innerTxs[0].Gas, "Gas should be the same as the gas in transaction receipt for getValue call")

		// Validate second inner transaction: ContractA -> ContractB (dummy call)
		expectedInnertx2 := &types.InnerTx{
			Dept:          *big.NewInt(1),
			InternalIndex: *big.NewInt(0),
			CallType:      "call",
			Name:          "call_0",
			TraceAddress:  "",
			CodeAddress:   "",
			From:          ContractAAddr.Hex(),
			To:            ContractBAddr.Hex(),
			Input:         "0x32e43a11",
			Output:        "",
			IsError:       false,
			Error:         "",
			Value:         "",
			ValueWei:      "0",
			CallValueWei:  "0x0",
		}
		ValidateInnerTransactionMatch(t, innerTxs[1], expectedInnertx2, "Second inner transaction (ContractA -> ContractB)")

		// gasUsed of second inner transaction should be less than the gasUsed in transaction receipt
		require.Less(t, innerTxs[1].GasUsed, contractAReceipt.GasUsed, "Second inner transaction GasUsed should be the less than contractAReceipt.GasUsed")
	})

	t.Run("GetInternalTransactions_WithOutput", func(t *testing.T) {
		innerTxs, err := operations.EthGetInternalTransactions(signedContractCGetValueTxHash)
		require.NoError(t, err)
		require.NotNil(t, innerTxs, "Inner transactions result should not be nil")
		require.Len(t, innerTxs, 1, "Should have exactly 1 inner transaction for getValue call")

		innerTx := innerTxs[0]
		require.NotNil(t, innerTx, "innerTx should not be nil")

		expectedInnerTx := &types.InnerTx{
			Dept:          *big.NewInt(0),
			InternalIndex: *big.NewInt(0),
			CallType:      "",
			Name:          "",
			TraceAddress:  "",
			CodeAddress:   "",
			From:          preexecFrom.Hex(),
			To:            ContractCAddr.Hex(),
			Input:         "",
			Output:        "0x0000000000000000000000000000000000000000000000000000000000000123",
			IsError:       false,
			Error:         "",
			Value:         "",
			ValueWei:      "0",
			CallValueWei:  "0x0",
		}

		ValidateInnerTransactionMatch(t, innerTx, expectedInnerTx, "ContractC getValue inner transaction with output")
		require.Equal(t, contractCGetValueReceipt.GasUsed, innerTx.GasUsed, "GasUsed should be the same as the gasUsed in transaction receipt")
	})
	t.Run("GetInternalTransactions_Batch", func(t *testing.T) {
		// Send multiple transactions in a batch
		txHashes := TransTokenBatch(t, ctx, client, uint256.NewInt(params.GWei), operations.DefaultL2NewAcc1Address, 10, operations.DefaultL2AdminPrivateKey)
		require.Len(t, txHashes, 10, "Should have created 10 transactions")

		// Verify each transaction has exactly 1 inner transaction
		numInnerTxs := 0
		for i, txHash := range txHashes {
			fmt.Printf("Getting inner transactions for tx %d: %s\n", i, txHash)
			innerTxs, err := operations.EthGetInternalTransactions(common.HexToHash(txHash))
			require.NoError(t, err, "Failed to get inner transactions for tx %d: %s", i, txHash)
			require.Len(t, innerTxs, 1, "Transaction %d (%s) should have exactly 1 inner transaction, got %d", i, txHash, len(innerTxs))

			txReceipt, err := client.TransactionReceipt(ctx, common.HexToHash(txHash))
			require.NoError(t, err, "Failed to get transaction receipt for tx %d: %s", i, txHash)

			innerTx := innerTxs[0]
			require.NotNil(t, innerTx, "innerTx should not be nil for tx %d", i)

			expectedInnerTx := &types.InnerTx{
				Dept:          *big.NewInt(0),
				InternalIndex: *big.NewInt(0),
				CallType:      "",
				Name:          "",
				TraceAddress:  "",
				CodeAddress:   "",
				From:          operations.DefaultL1AdminAddress,
				To:            operations.DefaultL2NewAcc1Address,
				Input:         "",
				Output:        "",
				IsError:       false,
				Error:         "",
				Value:         "",
				ValueWei:      strconv.FormatUint(params.GWei, 10),
				CallValueWei:  hexutil.EncodeUint64(params.GWei),
			}

			ValidateInnerTransactionMatch(t, innerTx, expectedInnerTx, fmt.Sprintf("Batch transaction %d", i))
			require.Equal(t, txReceipt.GasUsed, innerTx.GasUsed, "GasUsed should be the same as the gasUsed in transaction receipt for batch transaction %d", i)
			numInnerTxs++
		}
		require.Equal(t, 10, numInnerTxs, "Should have 10 inner transactions for all 10 transactions")
	})

	t.Run("GetBlockInternalTransactions_Batch", func(t *testing.T) {
		// Send multiple transactions in a batch
		txHashes := TransTokenBatch(t, ctx, client, uint256.NewInt(params.GWei), operations.DefaultL2NewAcc1Address, 5, operations.DefaultL2AdminPrivateKey)
		require.Len(t, txHashes, 5, "Should have created 5 transactions")

		blockNumbers := make(map[uint64][]int)

		for i, txHashStr := range txHashes {
			txHash := common.HexToHash(txHashStr)
			receipt, err := client.TransactionReceipt(ctx, txHash)
			require.NoError(t, err, "Failed to get receipt for tx %d", i)

			blockNum := receipt.BlockNumber.Uint64()
			blockNumbers[blockNum] = append(blockNumbers[blockNum], i)
		}

		totalValidatedTxs := 0

		for blockNum, txIndices := range blockNumbers {
			fmt.Printf("Testing block %d with %d transactions\n", blockNum, len(txIndices))

			blockInnerTxs, err := operations.EthGetBlockInternalTransactions(rpc.BlockNumber(blockNum))
			require.NoError(t, err, "Failed to get block internal transactions for block %d", blockNum)
			require.NotNil(t, blockInnerTxs, "Block inner transactions should not be nil for block %d", blockNum)

			// Verify all transactions in this block are present in block internal transactions
			batchTxsInBlock := 0
			for _, txIdx := range txIndices {
				txHash := common.HexToHash(txHashes[txIdx])
				blockInnerTxsForTx, exists := blockInnerTxs[txHash]
				require.True(t, exists, "Transaction %d (%s) should be in block %d internal transactions", txIdx, txHashes[txIdx], blockNum)
				require.Len(t, blockInnerTxsForTx, 1, "Transaction %d should have exactly 1 inner transaction", txIdx)
				batchTxsInBlock++

				innerTx := blockInnerTxsForTx[0]

				// Compare with individual transaction inner transactions
				individualInnerTxs, err := operations.EthGetInternalTransactions(txHash)
				require.NoError(t, err, "Failed to get individual inner transactions for tx %d", txIdx)
				require.Len(t, individualInnerTxs, 1, "Individual inner transactions should have 1 entry for tx %d", txIdx)

				ValidateInnerTransactionMatch(t, individualInnerTxs[0], innerTx, fmt.Sprintf("batch tx %d comparison", txIdx))
			}

			totalValidatedTxs += batchTxsInBlock
		}

		require.Equal(t, len(txHashes), totalValidatedTxs, "Should have validated all batch transactions")
		fmt.Printf("Successfully validated all %d transactions across %d blocks\n", totalValidatedTxs, len(blockNumbers))
	})

	t.Run("GetInnerTransactions_FailedTransactions", func(t *testing.T) {
		amount := uint256.NewInt(params.GWei)
		toAddress := ContractAAddr.String()

		txHash := TransTokenFail(t, ctx, client, TmpSenderPrivateKey, amount, toAddress)

		innerTxs, err := operations.EthGetInternalTransactions(txHash)
		require.NoError(t, err, "Should be able to get inner transactions")
		require.Len(t, innerTxs, 1, "Should have exactly 1 inner transaction for failed transfer")

		txReceipt, err := client.TransactionReceipt(ctx, txHash)
		require.NoError(t, err, "Should be able to get transaction receipt")

		innerTx := innerTxs[0]
		require.NotNil(t, innerTx, "innerTx should not be nil")

		senderPrivateKey, err := crypto.HexToECDSA(TmpSenderPrivateKey)
		require.NoError(t, err)
		senderAddress := crypto.PubkeyToAddress(senderPrivateKey.PublicKey)

		expectedInnerTx := &types.InnerTx{
			Dept:          *big.NewInt(0),
			InternalIndex: *big.NewInt(0),
			CallType:      "",
			Name:          "",
			TraceAddress:  "",
			CodeAddress:   "",
			From:          senderAddress.Hex(),
			To:            toAddress,
			Input:         "",
			Output:        "",
			IsError:       true,
			Error:         "execution reverted",
			Value:         "",
			ValueWei:      strconv.FormatUint(amount.Uint64(), 10),
			CallValueWei:  hexutil.EncodeUint64(amount.Uint64()),
		}

		ValidateInnerTransactionMatch(t, innerTx, expectedInnerTx, "Failed transaction inner transaction")
		require.Equal(t, txReceipt.GasUsed, innerTx.GasUsed, "GasUsed should be the same as the gasUsed in transaction receipt for failed transaction")
	})

	t.Run("SpecialBlockNumberFormats", func(t *testing.T) {
		// Test "latest" format
		latestResult, err := operations.EthGetBlockInternalTransactions(rpc.LatestBlockNumber)
		require.NoError(t, err)
		require.NotNil(t, latestResult, "Latest block should return a valid map")
		fmt.Printf("'latest' block contains %d transactions with inner transactions\n", len(latestResult))

		// Test "earliest" format
		earliestResult, err := operations.EthGetBlockInternalTransactions(rpc.EarliestBlockNumber)
		require.NoError(t, err)
		require.NotNil(t, earliestResult, "Earliest block should return a valid map")
		fmt.Printf("'earliest' block contains %d transactions with inner transactions\n", len(earliestResult))
	})
}

func TestTransactionPreExec(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	EnsureContractsDeployed(t)

	contractAABI, err := abi.JSON(strings.NewReader(constants.ContractAABIJson))
	require.NoError(t, err)
	calldata, err := contractAABI.Pack("triggerCall")
	require.NoError(t, err)

	ethClient, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	defer ethClient.Close()

	fromAddr := common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")

	// Fund the test address for gas validation tests
	ctx := context.Background()
	fundingAmount := uint256.NewInt(5000000000000000000)
	fundingTxHash := TransToken(t, ctx, ethClient, fundingAmount, fromAddr.String())
	t.Logf("Funded test address %s with 5 ETH, tx: %s", fromAddr.Hex(), fundingTxHash)

	t.Run("InnerTransactionTracking", func(t *testing.T) {
		// Test contract call with inner transactions
		txRequest := map[string]interface{}{
			"from": fromAddr.Hex(), "to": ContractAAddr.Hex(), "gas": "0x30000",
			"gasPrice": "0x4a817c800", "value": "0x0", "nonce": "0x1",
			"data": fmt.Sprintf("0x%x", calldata),
		}
		stateOverride := map[string]interface{}{
			fromAddr.Hex(): map[string]interface{}{"balance": "0x1000000000000000000000"},
		}

		result, err := operations.EthTransactionPreExec(txRequest, "latest", stateOverride)
		require.NoError(t, err)

		preExecResults, ok := result.([]interface{})
		require.True(t, ok, "Expected result to be an array")
		require.Len(t, preExecResults, 1)

		preExecResult, ok := preExecResults[0].(map[string]interface{})
		require.True(t, ok, "Expected preExecResult to be a map")
		require.NotNil(t, preExecResult["logs"])
		require.NotNil(t, preExecResult["stateDiff"])
		require.NotNil(t, preExecResult["gasUsed"])
		require.NotNil(t, preExecResult["blockNumber"])

		innerTxList, ok := preExecResult["innerTxs"].([]interface{})
		require.True(t, ok)
		require.GreaterOrEqual(t, len(innerTxList), 1)

		firstInnerTx := innerTxList[0].(map[string]interface{})
		require.Equal(t, "call", firstInnerTx["call_type"])
		require.Equal(t, strings.ToLower(ContractAAddr.Hex()), strings.ToLower(common.HexToAddress(firstInnerTx["to"].(string)).Hex()))
		require.Equal(t, "0xf18c388a", firstInnerTx["input"].(string))
		require.False(t, firstInnerTx["is_error"].(bool), "Expected is_error to be false for the first inner transaction")

		secondInnerTx := innerTxList[1].(map[string]interface{})
		require.Equal(t, "call", secondInnerTx["call_type"])
		require.Equal(t, strings.ToLower(ContractAAddr.Hex()), strings.ToLower(common.HexToAddress(secondInnerTx["from"].(string)).Hex()))
		require.Equal(t, strings.ToLower(ContractBAddr.Hex()), strings.ToLower(common.HexToAddress(secondInnerTx["to"].(string)).Hex()))
		require.Equal(t, "0x32e43a11", secondInnerTx["input"].(string))

		name := secondInnerTx["name"].(string)
		require.True(t, name[len(name)-1] >= '0' && name[len(name)-1] <= '9')
		require.False(t, secondInnerTx["is_error"].(bool), "Expected is_error to be false for the second inner transaction")
	})

	t.Run("GasValidation", func(t *testing.T) {
		balance, err := ethClient.BalanceAt(ctx, fromAddr, nil)
		require.NoError(t, err)
		balanceETH := new(big.Float).Quo(new(big.Float).SetInt(balance), new(big.Float).SetFloat64(1e18))
		t.Logf("Test address balance: %s ETH", balanceETH.String())

		t.Run("ContractCall", func(t *testing.T) {
			txRequest := map[string]interface{}{
				"from": fromAddr.Hex(), "to": ContractAAddr.Hex(), "gas": "0x100000",
				"gasPrice": "0x4a817c800", "value": "0x0", "nonce": "0x1",
				"data": fmt.Sprintf("0x%x", calldata),
			}

			// Get gasUsed from eth_transactionPreExec
			result, err := operations.EthTransactionPreExec(txRequest, "latest", nil)
			require.NoError(t, err)
			resultSlice := result.([]interface{})
			validationResult := ValidateResult(t, resultSlice[0], "gas_comparison_contract_call")
			preExecGasUsed := validationResult.GasUsed

			// Get gas estimate from eth_estimateGas
			estimateGasRequest := map[string]interface{}{
				"from": fromAddr.Hex(), "to": ContractAAddr.Hex(),
				"data": fmt.Sprintf("0x%x", calldata),
			}

			var estimateResult string
			err = ethClient.Client().Call(&estimateResult, "eth_estimateGas", estimateGasRequest, "latest")
			require.NoError(t, err)

			estimatedGas, err := strconv.ParseUint(strings.TrimPrefix(estimateResult, "0x"), 16, 64)
			require.NoError(t, err)

			// Validation: Both should be very close
			tolerance := uint64(5000) // Allow 5K gas difference for binary search precision

			require.Greater(t, preExecGasUsed, uint64(21000), "Gas should be > 21000 for contract call")
			require.Greater(t, estimatedGas, uint64(21000), "Estimated gas should be > 21000 for contract call")

			diff := uint64(0)
			if estimatedGas > preExecGasUsed {
				diff = estimatedGas - preExecGasUsed
			} else {
				diff = preExecGasUsed - estimatedGas
			}

			require.LessOrEqual(t, diff, tolerance,
				"Gas difference too large: preExec=%d, estimate=%d, diff=%d",
				preExecGasUsed, estimatedGas, diff)
		})

		t.Run("SimpleTransfer", func(t *testing.T) {
			transferTx := map[string]interface{}{
				"from": fromAddr.Hex(), "to": "0x742d35Cc4cF52f9234E96bC29d7F6a0c91d87b06",
				"value": "0x1000000000000000", "gas": "0x5208", // 21000 in hex
				"gasPrice": "0x4a817c800", "nonce": "0x2",
			}

			// PreExec gas usage
			result, err := operations.EthTransactionPreExec(transferTx, "latest", nil)
			require.NoError(t, err)
			resultSlice := result.([]interface{})
			validationResult := ValidateResult(t, resultSlice[0], "gas_comparison_transfer")
			transferPreExecGas := validationResult.GasUsed

			// Estimate gas usage
			transferEstimate := map[string]interface{}{
				"from": fromAddr.Hex(), "to": "0x742d35Cc4cF52f9234E96bC29d7F6a0c91d87b06",
				"value": "0x1000000000000000",
			}
			var estimateResult string
			err = ethClient.Client().Call(&estimateResult, "eth_estimateGas", transferEstimate, "latest")
			require.NoError(t, err)
			transferEstimatedGas, err := strconv.ParseUint(strings.TrimPrefix(estimateResult, "0x"), 16, 64)
			require.NoError(t, err)

			// Simple transfers should be exactly 21000 gas
			require.Equal(t, uint64(21000), transferPreExecGas, "Simple transfer should use exactly 21000 gas")
			require.Equal(t, uint64(21000), transferEstimatedGas, "Simple transfer estimate should be exactly 21000 gas")
		})

		t.Run("CreateOperation", func(t *testing.T) {
			factoryABI, err := abi.JSON(strings.NewReader(constants.ContractFactoryABIJson))
			require.NoError(t, err)
			createCalldata, err := factoryABI.Pack("createSimpleStorage", big.NewInt(999))
			require.NoError(t, err)

			createTx := map[string]interface{}{
				"from": fromAddr.Hex(), "to": FactoryAddr.Hex(), "gas": "0x200000",
				"gasPrice": "0x4a817c800", "value": "0x0", "nonce": "0x3",
				"data": fmt.Sprintf("0x%x", createCalldata),
			}

			// PreExec gas usage for CREATE
			result, err := operations.EthTransactionPreExec(createTx, "latest", nil)
			require.NoError(t, err)
			resultSlice := result.([]interface{})
			validationResult := ValidateResult(t, resultSlice[0], "gas_comparison_create")
			createPreExecGas := validationResult.GasUsed

			// Estimate gas for CREATE
			createEstimate := map[string]interface{}{
				"from": fromAddr.Hex(), "to": FactoryAddr.Hex(),
				"data": fmt.Sprintf("0x%x", createCalldata),
			}
			var estimateResult string
			err = ethClient.Client().Call(&estimateResult, "eth_estimateGas", createEstimate, "latest")
			require.NoError(t, err)
			createEstimatedGas, err := strconv.ParseUint(strings.TrimPrefix(estimateResult, "0x"), 16, 64)
			require.NoError(t, err)

			createDiff := uint64(0)
			if createEstimatedGas > createPreExecGas {
				createDiff = createEstimatedGas - createPreExecGas
			} else {
				createDiff = createPreExecGas - createEstimatedGas
			}

			require.LessOrEqual(t, createDiff, uint64(50000),
				"CREATE gas difference too large: preExec=%d, estimate=%d, diff=%d",
				createPreExecGas, createEstimatedGas, createDiff)
		})
	})

	t.Run("SimpleEthTransfer", func(t *testing.T) {
		transactionArgs := CreateBasicTransaction(
			"0x0165878a594ca255338adfa4d48449f69242eb8f",
			"0x1111111111111111111111111111111111111111",
			"0x0", "0x5208", "0x4a817c800", "0x0", "")

		stateOverrides := CreateDefaultStateOverrides()
		stateOverrides["0x0165878a594ca255338adfa4d48449f69242eb8f"].(map[string]interface{})["code"] = "0x608060405234801561001057600080fd5b50600436106100b45760003560e01c80638da5cb5b116100715780638da5cb5b1461013b57"

		result, err := operations.EthTransactionPreExec(transactionArgs, "latest", stateOverrides)
		require.NoError(t, err)
		resultSlice := result.([]interface{})
		validationResult := ValidateResult(t, resultSlice[0], "simple_eth_transfer")

		CheckSuccessfulResult(t, validationResult, "0x0165878a594ca255338adfa4d48449f69242eb8f", "simple ETH transfer")
		require.Empty(t, validationResult.InnerTxs, "Simple ETH transfer should not have inner transactions")
		require.Equal(t, validationResult.GasUsed, uint64(21000), "Simple ETH transfer should use exactly 21000 gas")
	})

	t.Run("EIP1559Transactions", func(t *testing.T) {
		testCases := []struct {
			name     string
			txFields map[string]interface{}
		}{
			{"maxFeePerGas_only", map[string]interface{}{"maxFeePerGas": "0x4a817c800"}},
			{"maxFeePerGas_with_maxPriorityFeePerGas", map[string]interface{}{
				"maxFeePerGas":         "0x4a817c800",
				"maxPriorityFeePerGas": "0x3b9aca00",
			}},
		}

		stateOverrides := CreateDefaultStateOverrides()

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create base transaction with EIP-1559 fields
				transactionArgs := CreateBasicTransaction(
					"0x0165878a594ca255338adfa4d48449f69242eb8f",
					"0x1111111111111111111111111111111111111111",
					"0x0", "0x5208", "0x4a817c800", "0x0", "")

				for key, value := range tc.txFields {
					transactionArgs[key] = value
				}

				// Remove gasPrice field for EIP-1559 transactions
				delete(transactionArgs, "gasPrice")

				result, err := operations.EthTransactionPreExec(transactionArgs, "latest", stateOverrides)
				require.NoError(t, err)
				resultSlice := result.([]interface{})
				validationResult := ValidateResult(t, resultSlice[0], tc.name)
				CheckSuccessfulResult(t, validationResult, "0x0165878a594ca255338adfa4d48449f69242eb8f", "Support EIP-1559 transactions")
				require.GreaterOrEqual(t, validationResult.GasUsed, uint64(21000), "Transaction should use more than 21000 gas")
				require.Greater(t, validationResult.BlockNumber.Uint64(), uint64(1), "Block Number should be greater than 1")
				require.Empty(t, validationResult.InnerTxs, "Inner Transactions should be empty")
				require.Empty(t, validationResult.Logs, "Logs should be empty")
			})
		}
	})

	t.Run("EIP7702Transactions", func(t *testing.T) {
		testCases := []struct {
			name      string
			addresses []string
		}{
			{"authorizationList_single", []string{"0x1111111111111111111111111111111111111111"}},
			{"authorizationList_multiple", []string{"0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222"}},
		}

		stateOverrides := CreateDefaultStateOverrides()

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create base transaction with authorization list
				transactionArgs := CreateBasicTransaction(
					"0x0165878a594ca255338adfa4d48449f69242eb8f",
					"0x1111111111111111111111111111111111111111",
					"0x0", "0x15f90", "0x4a817c800", "0x0", "")
				transactionArgs["authorizationList"] = CreateAuthorizationList(tc.addresses)

				result, err := operations.EthTransactionPreExec(transactionArgs, "latest", stateOverrides)
				require.NoError(t, err)
				resultSlice := result.([]interface{})
				validationResult := ValidateResult(t, resultSlice[0], tc.name)
				CheckSuccessfulResult(t, validationResult, "0x0165878a594ca255338adfa4d48449f69242eb8f", "Support EIP-7702 transactions")
				require.GreaterOrEqual(t, validationResult.GasUsed, uint64(21000), "Transaction should use more than 21000 gas")
				require.Greater(t, validationResult.BlockNumber.Uint64(), uint64(1), "Block Number should be greater than 1")
				require.Empty(t, validationResult.InnerTxs, "Inner Transactions should be empty")
				require.Empty(t, validationResult.Logs, "Logs should be empty")
			})
		}
	})

	t.Run("ContractCallWithStateOverrides", func(t *testing.T) {
		contractBAddr := "0x2222222222222222222222222222222222222222"
		contractCAddr := "0x3333333333333333333333333333333333333333"

		stateOverrides := CreateDefaultStateOverrides()

		// Set up ContractB with ContractC's address in storage slot 1
		stateOverrides[contractBAddr] = map[string]interface{}{
			"code": "0x" + constants.ContractBBytecodeStr,
			"storage": map[string]interface{}{
				"0x0000000000000000000000000000000000000000000000000000000000000001": "0x0000000000000000000000003333333333333333333333333333333333333333", // ContractC address in slot 1
			},
		}

		// Set up ContractC with initial storage value
		stateOverrides[contractCAddr] = map[string]interface{}{
			"code": "0x" + constants.ContractCBytecodeStr,
			"storage": map[string]interface{}{
				"0x0000000000000000000000000000000000000000000000000000000000000000": "0x0000000000000000000000000000000000000000000000000000000000000064", // Initial value = 100
			},
		}

		transactionArgs := CreateBasicTransaction(
			"0x0165878a594ca255338adfa4d48449f69242eb8f",
			contractBAddr, "0x0", "0x200000", "0x4a817c800",
			"0x0", "0x32e43a11") // dummy() function selector

		result, err := operations.EthTransactionPreExec(transactionArgs, "latest", stateOverrides)
		require.NoError(t, err)
		resultSlice := result.([]interface{})
		validationResult := ValidateResult(t, resultSlice[0], "dummy_call")

		for _, txResult := range resultSlice {
			testName := "ContractCallWithStateOverrides"
			validationResult := ValidateResult(t, txResult, testName)

			CheckSuccessfulResult(t, validationResult, "0x0165878a594ca255338adfa4d48449f69242eb8f", testName)
		}

		require.Greater(t, validationResult.GasUsed, uint64(21000), "Contract call should use more than 21000 gas")
		require.Equal(t, 0, len(validationResult.InnerTxs), "Should have no inner transactions")
	})

	t.Run("NonceTooLow", func(t *testing.T) {
		// Test transactions with nonces lower than the account's current nonce
		transactions := []map[string]interface{}{
			CreateBasicTransaction(
				"0x0165878a594ca255338adfa4d48449f69242eb8f",
				"0x5fbdb2315678afecb367f032d93f642f64180aa3",
				"0x0", "0x30000", "0x4a817c800", "0x1", ""), // nonce = 1
			CreateBasicTransaction(
				"0x0165878a594ca255338adfa4d48449f69242eb8f",
				"0x5fbdb2315678afecb367f032d93f642f64180aa3",
				"0x0", "0x30000", "0x4a817c800", "0x2", ""), // nonce = 2
		}

		stateOverrides := map[string]interface{}{
			"0x0165878a594ca255338adfa4d48449f69242eb8f": map[string]interface{}{
				"balance": "0x56bc75e2d630eb20000",
				"code":    "0x608060405234801561001057600080fd5b50600436106100b45760003560e01c80638da5cb5b116100715780638da5cb5b1461013b57",
				"nonce":   "0x3", // Account nonce = 3
			},
		}

		var result []interface{}
		for _, tx := range transactions {
			txResult, err := operations.EthTransactionPreExec(tx, "latest", stateOverrides)
			require.NoError(t, err)
			txResultSlice := txResult.([]interface{})
			require.Len(t, txResultSlice, 1, "Expected single transaction result")
			result = append(result, txResultSlice[0])
		}

		for i, txResult := range result {
			testName := fmt.Sprintf("transaction_%d_nonce_too_low", i+1)
			validationResult := ValidateResult(t, txResult, testName)

			CheckErrorResult(t, validationResult, "nonce too low", testName)

			require.Equal(t, uint64(0), validationResult.GasUsed, "%s should use 0 gas when rejected", testName)
			require.Len(t, validationResult.InnerTxs, 0, "%s should have no inner transactions when rejected", testName)
		}
	})
}

func TestNewTransactionTypes(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	defer client.Close()

	EnsureContractsDeployed(t)

	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)

	privateKey, err := crypto.HexToECDSA(TmpSenderPrivateKey)
	require.NoError(t, err)
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	toAddress := common.HexToAddress("0x1111111111111111111111111111111111111111")

	fundingAmount := uint256.NewInt(10000000000000000000)
	fundingTxHash := TransToken(t, ctx, client, fundingAmount, fromAddress.String())
	t.Logf("Funded test address %s with 10 ETH, tx: %s", fromAddress.Hex(), fundingTxHash)

	t.Run("EIP1559Transactions", func(t *testing.T) {
		t.Run("SimpleTransferWithMaxFeePerGas", func(t *testing.T) {
			nonce, err := client.PendingNonceAt(ctx, fromAddress)
			require.NoError(t, err)

			value := big.NewInt(1000000000000000000)
			gasLimit := uint64(21000)
			maxFeePerGas := big.NewInt(20000000000)
			maxPriorityFeePerGas := big.NewInt(1000000000)

			tx := types.NewTx(&types.DynamicFeeTx{
				ChainID:   chainID,
				Nonce:     nonce,
				GasTipCap: maxPriorityFeePerGas,
				GasFeeCap: maxFeePerGas,
				Gas:       gasLimit,
				To:        &toAddress,
				Value:     value,
				Data:      nil,
			})

			txHash, receipt := SignAndSendTransaction(ctx, t, client, tx, privateKey)

			require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "Transaction should succeed")
			require.Equal(t, uint64(21000), receipt.GasUsed, "Should use exactly 21000 gas for simple transfer")
			require.Equal(t, uint8(types.DynamicFeeTxType), receipt.Type, "Receipt type should be DynamicFeeTx")

			t.Logf("EIP-1559 transfer successful: %s, gas used: %d", txHash.Hex(), receipt.GasUsed)
		})

		t.Run("ContractCallWithEIP1559", func(t *testing.T) {
			// Test EIP-1559 transaction calling a smart contract
			nonce, err := client.PendingNonceAt(ctx, fromAddress)
			require.NoError(t, err)

			contractAABI, err := abi.JSON(strings.NewReader(constants.ContractAABIJson))
			require.NoError(t, err)
			calldata, err := contractAABI.Pack("triggerCall")
			require.NoError(t, err)

			gasLimit := uint64(200000)
			maxFeePerGas := big.NewInt(20000000000)
			maxPriorityFeePerGas := big.NewInt(2000000000)

			// Create EIP-1559 contract call transaction
			tx := types.NewTx(&types.DynamicFeeTx{
				ChainID:   chainID,
				Nonce:     nonce,
				GasTipCap: maxPriorityFeePerGas,
				GasFeeCap: maxFeePerGas,
				Gas:       gasLimit,
				To:        &ContractAAddr,
				Value:     big.NewInt(0),
				Data:      calldata,
			})

			txHash, receipt := SignAndSendTransaction(ctx, t, client, tx, privateKey)

			require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "Contract call should succeed")
			require.Greater(t, receipt.GasUsed, uint64(21000), "Contract call should use more than 21000 gas")

			t.Logf("EIP-1559 contract call successful: %s, gas used: %d", txHash.Hex(), receipt.GasUsed)
		})

		t.Run("eth_feeHistory", func(t *testing.T) {
			numberOfTransactions := 10
			transErc20TokenBatch(t, ctx, client, ERC20Addr, big.NewInt(Gwei), toAddress.String(), numberOfTransactions)

			// Get fee history for last 20 blocks
			blockCount := uint64(20)
			history, err := client.FeeHistory(ctx, blockCount, nil, nil)
			require.NoError(t, err)

			oldestBlockNum := history.OldestBlock.Uint64()
			t.Logf("Fee history oldest block: %d, blocks returned: %d", oldestBlockNum, blockCount)

			for i := uint64(0); i < blockCount; i++ {
				blockNum := big.NewInt(int64(oldestBlockNum) + int64(i))
				block, err := client.BlockByNumber(ctx, blockNum)
				require.NoError(t, err, "Failed to get block %d", blockNum.Uint64())

				feeHistoryBaseFee := history.BaseFee[i]
				blockHeaderBaseFee := block.BaseFee()
				require.Equal(t, feeHistoryBaseFee, blockHeaderBaseFee, "Base fee should match between fee history and block headers")
			}
		})

	})

	t.Run("EIP2930Transaction", func(t *testing.T) {
		nonce, err := client.PendingNonceAt(ctx, fromAddress)
		require.NoError(t, err)

		gasLimit := uint64(26000)
		gasPrice, err := client.SuggestGasPrice(ctx)
		require.NoError(t, err)

		// Create access list
		accessList := types.AccessList{
			{
				Address: toAddress,
				StorageKeys: []common.Hash{
					common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
				},
			},
		}

		tx := types.NewTx(&types.AccessListTx{
			ChainID:    chainID,
			Nonce:      nonce,
			GasPrice:   gasPrice,
			Gas:        gasLimit,
			To:         &toAddress,
			Value:      big.NewInt(0),
			Data:       nil,
			AccessList: accessList,
		})

		txHash, receipt := SignAndSendTransaction(ctx, t, client, tx, privateKey)

		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "EIP-2930 transaction with access list should succeed")
		require.Equal(t, uint8(types.AccessListTxType), receipt.Type, "Receipt type should be AccessListTx")

		t.Logf("EIP-2930 AccessListTx successful: %s, gas used: %d", txHash.Hex(), receipt.GasUsed)
	})

	t.Run("EIP7702Transaction", func(t *testing.T) {
		nonce, err := client.PendingNonceAt(ctx, fromAddress)
		require.NoError(t, err)

		// Create authorization to delegate the fromAddress's code to ContractC
		auth := types.SetCodeAuthorization{
			ChainID: *uint256.MustFromBig(chainID),
			Address: ContractCAddr,
			Nonce:   nonce,
		}

		signedAuth, err := types.SignSetCode(privateKey, auth)
		require.NoError(t, err)

		authority, err := signedAuth.Authority()
		require.NoError(t, err)
		require.Equal(t, fromAddress, authority, "Authority should match the signing address")

		gasLimit := uint64(100000)
		maxFeePerGas := big.NewInt(20000000000)
		maxPriorityFeePerGas := big.NewInt(1000000000)

		// Set the fromAddress to delegate to ContractC's code
		tx := types.NewTx(&types.SetCodeTx{
			ChainID:   uint256.MustFromBig(chainID),
			Nonce:     nonce,
			GasTipCap: uint256.MustFromBig(maxPriorityFeePerGas),
			GasFeeCap: uint256.MustFromBig(maxFeePerGas),
			Gas:       gasLimit,
			To:        toAddress,
			Value:     uint256.NewInt(0),
			Data:      nil,
			AuthList:  []types.SetCodeAuthorization{signedAuth},
		})

		require.Equal(t, uint8(types.SetCodeTxType), tx.Type(), "Transaction type should be SetCodeTx")

		authList := tx.SetCodeAuthorizations()
		require.NotNil(t, authList, "Authorization list should not be nil")
		require.Len(t, authList, 1, "Should have exactly 1 authorization")
		require.Equal(t, ContractCAddr, authList[0].Address, "Authorization should delegate to ContractC")

		txHash, receipt := SignAndSendTransaction(ctx, t, client, tx, privateKey)

		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "EIP-7702 transaction should succeed")
		require.Equal(t, uint8(types.SetCodeTxType), receipt.Type, "Receipt type should be SetCodeTx")
		require.Greater(t, receipt.GasUsed, uint64(21000), "Should use more than 21000 gas due to authorization processing")

		minedTx, _, err := client.TransactionByHash(ctx, txHash)
		require.NoError(t, err)

		txSigner := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), big.NewInt(1), 0)
		txFromAddress, err := types.Sender(txSigner, minedTx)
		require.NoError(t, err)
		require.Equal(t, fromAddress, txFromAddress, "Transaction from address should match the EOA address")
		require.Equal(t, toAddress, *minedTx.To(), "Transaction to address should match the recipient address")

		t.Logf("EIP-7702 transaction mined successfully in block %d, gas used: %d", receipt.BlockNumber.Uint64(), receipt.GasUsed)
	})

	t.Run("EIP3198Transaction", func(t *testing.T) {
		// BASEFEE (0x48) + STOP (0x00)
		var result hexutil.Bytes

		err := client.Client().CallContext(ctx, &result, "eth_call", map[string]interface{}{
			"to":   nil,      // Contract creation context
			"data": "0x4800", // BASEFEE + STOP
		}, "latest")
		require.NoError(t, err, "BASEFEE opcode should execute without error (EIP-3198 is supported)")

		latestBlock, err := client.BlockByNumber(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, latestBlock, "Block should not be nil")
		require.NotNil(t, latestBlock.BaseFee(), "Block should have a base fee field (EIP-3198)")

		t.Logf("Current block %d base fee: %s wei", latestBlock.Number().Uint64(), latestBlock.BaseFee().String())
	})

	t.Run("EIP3529Transaction", func(t *testing.T) {
		// Constructor bytecode: CALLER (0x33), SELFDESTRUCT (0xff)
		// This will self-destruct immediately during deployment, sending any value to the caller
		deployBytecode := common.Hex2Bytes("33ff")

		nonce, err := client.PendingNonceAt(ctx, fromAddress)
		require.NoError(t, err)

		gasPrice, err := client.SuggestGasPrice(ctx)
		require.NoError(t, err)

		deployTx := types.NewContractCreation(nonce, big.NewInt(0), 100000, gasPrice, deployBytecode)

		txHash, deployReceipt := SignAndSendTransaction(ctx, t, client, deployTx, privateKey)

		contractAddress := deployReceipt.ContractAddress
		t.Logf("Contract address (self-destructed): %s, tx hash: %s", contractAddress.Hex(), txHash.Hex())

		// Use debug_traceTransaction to get the refund counter
		var traceResult map[string]interface{}
		err = client.Client().CallContext(ctx, &traceResult, "debug_traceTransaction", txHash, map[string]interface{}{})
		require.NoError(t, err)

		refundCounter := GetRefundCounterFromTrace(traceResult, "SELFDESTRUCT")
		t.Logf("Refund counter after SELFDESTRUCT: %d", refundCounter)

		require.Equal(t, uint64(0), refundCounter, "SELFDESTRUCT refund should be 0 with EIP-3529")

		codeAfter, err := client.CodeAt(ctx, contractAddress, deployReceipt.BlockNumber)
		require.NoError(t, err)
		require.Empty(t, codeAfter, "Contract should have no code after SELFDESTRUCT")
	})

	t.Run("EIP4844Transactions", func(t *testing.T) {
		// OP Stack disables EIP-4844 blob transactions on Layer 2
		// So we check if blob fields are present in block data
		result, err := operations.EthGetBlockByNumber("latest", true)
		require.NoError(t, err)

		blockdata, _ := result.(map[string]interface{})
		require.NotNil(t, blockdata, "Block data should not be nil")

		blockGasUsed, exists := blockdata["blobGasUsed"].(string)
		require.True(t, exists, "Block should have blockGasUsed field")

		excessBlobGas, exists := blockdata["excessBlobGas"].(string)
		require.True(t, exists, "Block should have excessBlobGas field")

		t.Logf("Block gas used: %s", blockGasUsed)
		t.Logf("Block excess blob gas: %s", excessBlobGas)
	})

}
