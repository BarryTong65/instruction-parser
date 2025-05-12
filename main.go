package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	lookup "github.com/gagliardetto/solana-go/programs/address-lookup-table"
	"github.com/gagliardetto/solana-go/rpc"
	"golang.org/x/time/rate"
)

// Jupiter V6 指令类型的判别码 (Discriminators)
var InstructionDiscriminators = map[string][]byte{
	"route":                              {0xE5, 0x17, 0xCB, 0x97, 0x7A, 0xE3, 0xAD, 0x2A},
	"routeWithTokenLedger":               {0x96, 0x56, 0x47, 0x74, 0xA7, 0x5D, 0x0E, 0x68},
	"sharedAccountsRoute":                {0xC1, 0x20, 0x9B, 0x33, 0x41, 0xD6, 0x9C, 0x81},
	"sharedAccountsRouteWithTokenLedger": {0xE6, 0x79, 0x8F, 0x50, 0x77, 0x9F, 0x6A, 0xAA},
	"exactOutRoute":                      {0xD0, 0x33, 0xEF, 0x97, 0x7B, 0x2B, 0xED, 0x5C},
	"sharedAccountsExactOutRoute":        {0xB0, 0xD1, 0x69, 0xA8, 0x9A, 0x7D, 0x45, 0x3E},
}

// 表示不同的交换协议类型
type SwapType string

const (
	SwapSaber                        SwapType = "Saber"
	SwapSaberAddDecimalsDeposit      SwapType = "SaberAddDecimalsDeposit"
	SwapSaberAddDecimalsWithdraw     SwapType = "SaberAddDecimalsWithdraw"
	SwapTokenSwap                    SwapType = "TokenSwap"
	SwapSencha                       SwapType = "Sencha"
	SwapStep                         SwapType = "Step"
	SwapCropper                      SwapType = "Cropper"
	SwapRaydium                      SwapType = "Raydium"
	SwapCrema                        SwapType = "Crema"
	SwapLifinity                     SwapType = "Lifinity"
	SwapMercurial                    SwapType = "Mercurial"
	SwapCykura                       SwapType = "Cykura"
	SwapSerum                        SwapType = "Serum"
	SwapMarinadeDeposit              SwapType = "MarinadeDeposit"
	SwapMarinadeUnstake              SwapType = "MarinadeUnstake"
	SwapAldrin                       SwapType = "Aldrin"
	SwapAldrinV2                     SwapType = "AldrinV2"
	SwapWhirlpool                    SwapType = "Whirlpool"
	SwapInvariant                    SwapType = "Invariant"
	SwapMeteora                      SwapType = "Meteora"
	SwapGooseFX                      SwapType = "GooseFX"
	SwapDeltaFi                      SwapType = "DeltaFi"
	SwapBalansol                     SwapType = "Balansol"
	SwapMarcoPolo                    SwapType = "MarcoPolo"
	SwapDradex                       SwapType = "Dradex"
	SwapLifinityV2                   SwapType = "LifinityV2"
	SwapRaydiumClmm                  SwapType = "RaydiumClmm"
	SwapOpenbook                     SwapType = "Openbook"
	SwapPhoenix                      SwapType = "Phoenix"
	SwapSymmetry                     SwapType = "Symmetry"
	SwapTokenSwapV2                  SwapType = "TokenSwapV2"
	SwapHeliumTreasuryManagement     SwapType = "HeliumTreasuryManagementRedeemV0"
	SwapStakeDexStakeWrappedSol      SwapType = "StakeDexStakeWrappedSol"
	SwapStakeDexSwapViaStake         SwapType = "StakeDexSwapViaStake"
	SwapGooseFXV2                    SwapType = "GooseFXV2"
	SwapPerps                        SwapType = "Perps"
	SwapPerpsAddLiquidity            SwapType = "PerpsAddLiquidity"
	SwapPerpsRemoveLiquidity         SwapType = "PerpsRemoveLiquidity"
	SwapMeteoraDlmm                  SwapType = "MeteoraDlmm"
	SwapOpenBookV2                   SwapType = "OpenBookV2"
	SwapRaydiumClmmV2                SwapType = "RaydiumClmmV2"
	SwapStakeDexPrefundWithdrawStake SwapType = "StakeDexPrefundWithdrawStakeAndDepositStake"
	SwapClone                        SwapType = "Clone"
	SwapSanctumS                     SwapType = "SanctumS"
	SwapSanctumSAddLiquidity         SwapType = "SanctumSAddLiquidity"
	SwapSanctumSRemoveLiquidity      SwapType = "SanctumSRemoveLiquidity"
	SwapRaydiumCP                    SwapType = "RaydiumCP"
	SwapWhirlpoolSwapV2              SwapType = "WhirlpoolSwapV2"
	SwapOneIntro                     SwapType = "OneIntro"
	SwapPumpdotfunWrappedBuy         SwapType = "PumpdotfunWrappedBuy"
	SwapPumpdotfunWrappedSell        SwapType = "PumpdotfunWrappedSell"
	SwapPerpsV2                      SwapType = "PerpsV2"
	SwapPerpsV2AddLiquidity          SwapType = "PerpsV2AddLiquidity"
	SwapPerpsV2RemoveLiquidity       SwapType = "PerpsV2RemoveLiquidity"
	SwapMoonshotWrappedBuy           SwapType = "MoonshotWrappedBuy"
	SwapMoonshotWrappedSell          SwapType = "MoonshotWrappedSell"
	SwapStabbleStableSwap            SwapType = "StabbleStableSwap"
	SwapStabbleWeightedSwap          SwapType = "StabbleWeightedSwap"
	SwapObric                        SwapType = "Obric"
	SwapFoxBuyFromEstimatedCost      SwapType = "FoxBuyFromEstimatedCost"
	SwapFoxClaimPartial              SwapType = "FoxClaimPartial"
	SwapSolFi                        SwapType = "SolFi"
	Woofi                            SwapType = "Woofi"
	SwapPumpdotfunAmmBuy             SwapType = "PumpdotfunAmmBuy"
	SwapPumpdotfunAmmSell            SwapType = "PumpdotfunAmmSell"
)

// 交换类型到索引的映射
var SwapTypeToIndex = map[SwapType]uint8{
	SwapSaber:                        0,
	SwapSaberAddDecimalsDeposit:      1,
	SwapSaberAddDecimalsWithdraw:     2,
	SwapTokenSwap:                    3,
	SwapSencha:                       4,
	SwapStep:                         5,
	SwapCropper:                      6,
	SwapRaydium:                      7,
	SwapCrema:                        8,
	SwapLifinity:                     9,
	SwapMercurial:                    10,
	SwapCykura:                       11,
	SwapSerum:                        12,
	SwapMarinadeDeposit:              13,
	SwapMarinadeUnstake:              14,
	SwapAldrin:                       15,
	SwapAldrinV2:                     16,
	SwapWhirlpool:                    17,
	SwapInvariant:                    18,
	SwapMeteora:                      19,
	SwapGooseFX:                      20,
	SwapDeltaFi:                      21,
	SwapBalansol:                     22,
	SwapMarcoPolo:                    23,
	SwapDradex:                       24,
	SwapLifinityV2:                   25,
	SwapRaydiumClmm:                  26,
	SwapOpenbook:                     27,
	SwapPhoenix:                      28,
	SwapSymmetry:                     29,
	SwapTokenSwapV2:                  30,
	SwapHeliumTreasuryManagement:     31,
	SwapStakeDexStakeWrappedSol:      32,
	SwapStakeDexSwapViaStake:         33,
	SwapGooseFXV2:                    34,
	SwapPerps:                        35,
	SwapPerpsAddLiquidity:            36,
	SwapPerpsRemoveLiquidity:         37,
	SwapMeteoraDlmm:                  38,
	SwapOpenBookV2:                   39,
	SwapRaydiumClmmV2:                40,
	SwapStakeDexPrefundWithdrawStake: 41,
	SwapClone:                        42,
	SwapSanctumS:                     43,
	SwapSanctumSAddLiquidity:         44,
	SwapSanctumSRemoveLiquidity:      45,
	SwapRaydiumCP:                    46,
	SwapWhirlpoolSwapV2:              47,
	SwapOneIntro:                     48,
	SwapPumpdotfunWrappedBuy:         49,
	SwapPumpdotfunWrappedSell:        50,
	SwapPerpsV2:                      51,
	SwapPerpsV2AddLiquidity:          52,
	SwapPerpsV2RemoveLiquidity:       53,
	SwapMoonshotWrappedBuy:           54,
	SwapMoonshotWrappedSell:          55,
	SwapStabbleStableSwap:            56,
	SwapStabbleWeightedSwap:          57,
	SwapObric:                        58,
	SwapFoxBuyFromEstimatedCost:      59,
	SwapFoxClaimPartial:              60,
	SwapSolFi:                        61,
	Woofi:                            76,
	SwapPumpdotfunAmmBuy:             108,
	SwapPumpdotfunAmmSell:            109,
}

// Swap 结构体
type Swap struct {
	Type   SwapType               `json:"name"`
	Params map[string]interface{} `json:"params"`
}

// RoutePlanStep 表示路由计划中的一个步骤
type RoutePlanStep struct {
	Swap        Swap  `json:"swap"`
	Percent     uint8 `json:"percent"`
	InputIndex  uint8 `json:"input_index"`
	OutputIndex uint8 `json:"output_index"`
}

// JupiterSwapParams 表示 Jupiter 交换参数
type JupiterSwapParams struct {
	InstructionType string          `json:"instruction_type"`
	ID              uint8           `json:"id,omitempty"`
	RoutePlan       []RoutePlanStep `json:"route_plan"`
	InAmount        uint64          `json:"in_amount,omitempty"`
	OutAmount       uint64          `json:"out_amount,omitempty"`
	QuotedOutAmount uint64          `json:"quoted_out_amount,omitempty"`
	QuotedInAmount  uint64          `json:"quoted_in_amount,omitempty"`
	SlippageBps     uint16          `json:"slippage_bps"`
	PlatformFeeBps  uint8           `json:"platform_fee_bps"`
	MinAmountOut    uint64          `json:"min_amount_out,omitempty"`
}

// Jupiter V6 Program ID
var jupiterV6ProgramID = solana.MustPublicKeyFromBase58("JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4")

func main() {
	// Transaction signature
	txSignature := solana.MustSignatureFromBase58("iFqs11xumLUiicLW36TzJNoJVnRdu5EtXXLsA9imFfUMonDvnErppnpm5BnXSAwUsyvWhG7EFMp3aYP7suqjYPg")

	// Initialize RPC client with rate limiting
	rpcClient := rpc.NewWithCustomRPCClient(rpc.NewWithLimiter(
		rpc.MainNetBeta.RPC,
		rate.Every(time.Second),
		5,
	))

	// Get transaction with version support
	version := uint64(0)
	tx, err := rpcClient.GetTransaction(
		context.Background(),
		txSignature,
		&rpc.GetTransactionOpts{
			MaxSupportedTransactionVersion: &version,
			Encoding:                       solana.EncodingBase64,
		},
	)
	if err != nil {
		fmt.Printf("Error getting transaction: %v\n", err)
		return
	}

	// Parse the transaction
	parsedTx, err := tx.Transaction.GetTransaction()
	if err != nil {
		fmt.Printf("Error parsing transaction: %v\n", err)
		return
	}

	// Process versioned transactions with address lookup tables
	if parsedTx.Message.IsVersioned() {
		err = resolveAddressLookupTables(parsedTx, rpcClient)
		if err != nil {
			fmt.Printf("Error resolving address lookup tables: %v\n", err)
			return
		}
	}

	// Debug print the transaction
	fmt.Println("Transaction details:")
	fmt.Printf("  Instructions count: %d\n", len(parsedTx.Message.Instructions))
	fmt.Printf("  Is versioned: %v\n", parsedTx.Message.IsVersioned())

	// Now find and parse Jupiter instructions
	found := false
	for i, inst := range parsedTx.Message.Instructions {
		// Safely access program ID
		programIDIndex := int(inst.ProgramIDIndex)
		if programIDIndex >= len(parsedTx.Message.AccountKeys) {
			fmt.Printf("Instruction %d: Program ID index out of range\n", i)
			continue
		}

		programID := parsedTx.Message.AccountKeys[programIDIndex]
		if programID.Equals(jupiterV6ProgramID) {
			found = true
			fmt.Printf("\nFound Jupiter instruction at index %d\n", i)
			fmt.Printf("Data length: %d bytes\n", len(inst.Data))

			if len(inst.Data) > 0 {
				// Print raw bytes for debugging
				fmt.Printf("Raw data (First 32 bytes): %X\n", inst.Data[:min(32, len(inst.Data))])

				// Parse the instruction
				result, err := parseJupiterV6Instruction(inst.Data)
				if err != nil {
					fmt.Printf("Error parsing Jupiter V6 instruction: %v\n", err)
					continue
				}

				// Print detailed results
				printJupiterV6Results(result)
			}
		}
	}

	if !found {
		fmt.Println("No Jupiter instructions found in main instructions")
		// Check inner instructions if available
		if tx.Meta != nil && tx.Meta.InnerInstructions != nil {
			for _, innerInst := range tx.Meta.InnerInstructions {
				for i, inst := range innerInst.Instructions {
					if inst.ProgramIDIndex < uint16(len(parsedTx.Message.AccountKeys)) {
						programID := parsedTx.Message.AccountKeys[inst.ProgramIDIndex]
						if programID.Equals(jupiterV6ProgramID) {
							fmt.Printf("\nFound Jupiter instruction in inner instruction at index %d\n", i)
							// 处理内部指令...
						}
					}
				}
			}
		}
	}
}

// parseJupiterV6Instruction 解析 Jupiter V6 指令数据
func parseJupiterV6Instruction(data []byte) (*JupiterSwapParams, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("instruction data too short")
	}

	// Check discriminator to determine instruction type
	discriminator := data[:8]

	// 检查各种指令类型
	if bytesEqual(discriminator, InstructionDiscriminators["route"]) {
		return parseRouteInstruction(data, "route")
	} else if bytesEqual(discriminator, InstructionDiscriminators["routeWithTokenLedger"]) {
		return parseRouteInstruction(data, "routeWithTokenLedger")
	} else if bytesEqual(discriminator, InstructionDiscriminators["sharedAccountsRoute"]) {
		return parseSharedAccountsRoute(data, "sharedAccountsRoute")
	} else if bytesEqual(discriminator, InstructionDiscriminators["sharedAccountsRouteWithTokenLedger"]) {
		return parseSharedAccountsRoute(data, "sharedAccountsRouteWithTokenLedger")
	} else if bytesEqual(discriminator, InstructionDiscriminators["exactOutRoute"]) {
		return parseExactOutRoute(data, "exactOutRoute")
	} else if bytesEqual(discriminator, InstructionDiscriminators["sharedAccountsExactOutRoute"]) {
		return parseSharedAccountsRoute(data, "sharedAccountsExactOutRoute")
	}

	return nil, fmt.Errorf("unknown instruction discriminator: %X", discriminator)
}

// parseRouteInstruction 解析 route 和 routeWithTokenLedger 指令
func parseRouteInstruction(data []byte, instructionType string) (*JupiterSwapParams, error) {
	offset := 8 // Skip discriminator

	// 解析 route plan 数量
	routePlanCount := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	// 解析每个 route plan step
	routePlan := make([]RoutePlanStep, routePlanCount)
	for i := uint32(0); i < routePlanCount; i++ {
		step, newOffset, err := parseRoutePlanStep(data, offset)
		if err != nil {
			return nil, fmt.Errorf("error parsing route plan step %d: %v", i, err)
		}
		routePlan[i] = step
		offset = newOffset
	}

	// 解析其他参数
	inAmount := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	quotedOutAmount := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	slippageBps := binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	platformFeeBps := data[offset]

	// 计算 min_amount_out
	minAmountOut := uint64(float64(quotedOutAmount) * (1.0 - float64(slippageBps)/10000.0))

	return &JupiterSwapParams{
		InstructionType: instructionType,
		RoutePlan:       routePlan,
		InAmount:        inAmount,
		QuotedOutAmount: quotedOutAmount,
		SlippageBps:     slippageBps,
		PlatformFeeBps:  platformFeeBps,
		MinAmountOut:    minAmountOut,
	}, nil
}

// parseSharedAccountsRoute 解析 sharedAccountsRoute 类型的指令
func parseSharedAccountsRoute(data []byte, instructionType string) (*JupiterSwapParams, error) {
	offset := 8 // Skip discriminator

	// 解析 ID
	id := data[offset]
	offset++

	// 解析 route plan 数量
	routePlanCount := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	// 解析每个 route plan step
	routePlan := make([]RoutePlanStep, routePlanCount)
	for i := uint32(0); i < routePlanCount; i++ {
		step, newOffset, err := parseRoutePlanStep(data, offset)
		if err != nil {
			return nil, fmt.Errorf("error parsing route plan step %d: %v", i, err)
		}
		routePlan[i] = step
		offset = newOffset
	}

	var inAmount, quotedOutAmount, minAmountOut uint64

	// 根据指令类型解析剩余字段
	if instructionType == "sharedAccountsExactOutRoute" {
		// exactOut 指令的结构不同
		quotedOutAmount = binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8

		inAmount = binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8

		slippageBps := binary.LittleEndian.Uint16(data[offset : offset+2])
		offset += 2

		platformFeeBps := data[offset]

		// 对于 exactOut，计算最大输入量
		maxAmountIn := uint64(float64(inAmount) * (1.0 + float64(slippageBps)/10000.0))

		return &JupiterSwapParams{
			InstructionType: instructionType,
			ID:              id,
			RoutePlan:       routePlan,
			OutAmount:       quotedOutAmount,
			QuotedInAmount:  inAmount,
			SlippageBps:     slippageBps,
			PlatformFeeBps:  platformFeeBps,
			MinAmountOut:    maxAmountIn, // 存储在这个字段中
		}, nil
	} else {
		// 标准的 route 指令
		inAmount = binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8

		quotedOutAmount = binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8

		slippageBps := binary.LittleEndian.Uint16(data[offset : offset+2])
		offset += 2

		platformFeeBps := data[offset]

		// 计算 min_amount_out
		minAmountOut = uint64(float64(quotedOutAmount) * (1.0 - float64(slippageBps)/10000.0))

		return &JupiterSwapParams{
			InstructionType: instructionType,
			ID:              id,
			RoutePlan:       routePlan,
			InAmount:        inAmount,
			QuotedOutAmount: quotedOutAmount,
			SlippageBps:     slippageBps,
			PlatformFeeBps:  platformFeeBps,
			MinAmountOut:    minAmountOut,
		}, nil
	}
}

// parseExactOutRoute 解析 exactOutRoute 指令
func parseExactOutRoute(data []byte, instructionType string) (*JupiterSwapParams, error) {
	offset := 8 // Skip discriminator

	// 解析 route plan 数量
	routePlanCount := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	// 解析每个 route plan step
	routePlan := make([]RoutePlanStep, routePlanCount)
	for i := uint32(0); i < routePlanCount; i++ {
		step, newOffset, err := parseRoutePlanStep(data, offset)
		if err != nil {
			return nil, fmt.Errorf("error parsing route plan step %d: %v", i, err)
		}
		routePlan[i] = step
		offset = newOffset
	}

	// exactOut 指令的结构
	outAmount := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	quotedInAmount := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	slippageBps := binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	platformFeeBps := data[offset]

	// 计算最大输入量
	maxAmountIn := uint64(float64(quotedInAmount) * (1.0 + float64(slippageBps)/10000.0))

	return &JupiterSwapParams{
		InstructionType: instructionType,
		RoutePlan:       routePlan,
		OutAmount:       outAmount,
		QuotedInAmount:  quotedInAmount,
		SlippageBps:     slippageBps,
		PlatformFeeBps:  platformFeeBps,
		MinAmountOut:    maxAmountIn, // 对于 exactOut，这实际上是最大输入量
	}, nil
}

// parseRoutePlanStep 解析单个路由计划步骤
func parseRoutePlanStep(data []byte, offset int) (RoutePlanStep, int, error) {
	if offset+4 > len(data) {
		return RoutePlanStep{}, offset, fmt.Errorf("not enough data for route plan step")
	}

	// 解析 swap type (1 byte)
	swapTypeIndex := data[offset]
	offset++

	// 根据索引确定 swap type 和参数
	swap, err := decodeSwapType(swapTypeIndex, data, offset)
	if err != nil {
		return RoutePlanStep{}, offset, err
	}

	// 更新 offset 基于 swap 类型的参数大小
	offset = updateOffsetForSwapType(swapTypeIndex, offset)

	// 解析 percent
	percent := data[offset]
	offset++

	// 解析 input_index
	inputIndex := data[offset]
	offset++

	// 解析 output_index
	outputIndex := data[offset]
	offset++

	return RoutePlanStep{
		Swap:        swap,
		Percent:     percent,
		InputIndex:  inputIndex,
		OutputIndex: outputIndex,
	}, offset, nil
}

// decodeSwapType 根据索引解码交换类型
func decodeSwapType(swapTypeIndex uint8, data []byte, offset int) (Swap, error) {
	switch swapTypeIndex {
	case 0:
		return Swap{Type: SwapSaber, Params: map[string]interface{}{}}, nil
	case 1:
		return Swap{Type: SwapSaberAddDecimalsDeposit, Params: map[string]interface{}{}}, nil
	case 2:
		return Swap{Type: SwapSaberAddDecimalsWithdraw, Params: map[string]interface{}{}}, nil
	case 3:
		return Swap{Type: SwapTokenSwap, Params: map[string]interface{}{}}, nil
	case 4:
		return Swap{Type: SwapSencha, Params: map[string]interface{}{}}, nil
	case 5:
		return Swap{Type: SwapStep, Params: map[string]interface{}{}}, nil
	case 6:
		return Swap{Type: SwapCropper, Params: map[string]interface{}{}}, nil
	case 7:
		return Swap{Type: SwapRaydium, Params: map[string]interface{}{}}, nil
	case 8:
		// Crema with a_to_b parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for Crema swap")
		}
		aToB := data[offset] != 0
		return Swap{Type: SwapCrema, Params: map[string]interface{}{"a_to_b": aToB}}, nil
	case 9:
		return Swap{Type: SwapLifinity, Params: map[string]interface{}{}}, nil
	case 10:
		return Swap{Type: SwapMercurial, Params: map[string]interface{}{}}, nil
	case 11:
		return Swap{Type: SwapCykura, Params: map[string]interface{}{}}, nil
	case 12:
		// Serum with side parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for Serum swap")
		}
		side := "Bid"
		if data[offset] != 0 {
			side = "Ask"
		}
		return Swap{Type: SwapSerum, Params: map[string]interface{}{"side": side}}, nil
	case 13:
		return Swap{Type: SwapMarinadeDeposit, Params: map[string]interface{}{}}, nil
	case 14:
		return Swap{Type: SwapMarinadeUnstake, Params: map[string]interface{}{}}, nil
	case 15:
		// Aldrin with side parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for Aldrin swap")
		}
		side := "Bid"
		if data[offset] != 0 {
			side = "Ask"
		}
		return Swap{Type: SwapAldrin, Params: map[string]interface{}{"side": side}}, nil
	case 16:
		// AldrinV2 with side parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for AldrinV2 swap")
		}
		side := "Bid"
		if data[offset] != 0 {
			side = "Ask"
		}
		return Swap{Type: SwapAldrinV2, Params: map[string]interface{}{"side": side}}, nil
	case 17:
		// Whirlpool with a_to_b parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for Whirlpool swap")
		}
		aToB := data[offset] != 0
		return Swap{Type: SwapWhirlpool, Params: map[string]interface{}{"a_to_b": aToB}}, nil
	case 18:
		// Invariant with x_to_y parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for Invariant swap")
		}
		xToY := data[offset] != 0
		return Swap{Type: SwapInvariant, Params: map[string]interface{}{"x_to_y": xToY}}, nil
	case 19:
		return Swap{Type: SwapMeteora, Params: map[string]interface{}{}}, nil
	case 20:
		return Swap{Type: SwapGooseFX, Params: map[string]interface{}{}}, nil
	case 21:
		// DeltaFi with stable parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for DeltaFi swap")
		}
		stable := data[offset] != 0
		return Swap{Type: SwapDeltaFi, Params: map[string]interface{}{"stable": stable}}, nil
	case 22:
		return Swap{Type: SwapBalansol, Params: map[string]interface{}{}}, nil
	case 23:
		// MarcoPolo with x_to_y parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for MarcoPolo swap")
		}
		xToY := data[offset] != 0
		return Swap{Type: SwapMarcoPolo, Params: map[string]interface{}{"x_to_y": xToY}}, nil
	case 24:
		// Dradex with side parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for Dradex swap")
		}
		side := "Bid"
		if data[offset] != 0 {
			side = "Ask"
		}
		return Swap{Type: SwapDradex, Params: map[string]interface{}{"side": side}}, nil
	case 25:
		return Swap{Type: SwapLifinityV2, Params: map[string]interface{}{}}, nil
	case 26:
		return Swap{Type: SwapRaydiumClmm, Params: map[string]interface{}{}}, nil
	case 27:
		// Openbook with side parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for Openbook swap")
		}
		side := "Bid"
		if data[offset] != 0 {
			side = "Ask"
		}
		return Swap{Type: SwapOpenbook, Params: map[string]interface{}{"side": side}}, nil
	case 28:
		// Phoenix with side parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for Phoenix swap")
		}
		side := "Bid"
		if data[offset] != 0 {
			side = "Ask"
		}
		return Swap{Type: SwapPhoenix, Params: map[string]interface{}{"side": side}}, nil
	case 29:
		// Symmetry with token IDs
		if offset+16 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for Symmetry swap")
		}
		fromTokenID := binary.LittleEndian.Uint64(data[offset : offset+8])
		toTokenID := binary.LittleEndian.Uint64(data[offset+8 : offset+16])
		return Swap{Type: SwapSymmetry, Params: map[string]interface{}{
			"from_token_id": fromTokenID,
			"to_token_id":   toTokenID,
		}}, nil
	case 30:
		return Swap{Type: SwapTokenSwapV2, Params: map[string]interface{}{}}, nil
	case 31:
		return Swap{Type: SwapHeliumTreasuryManagement, Params: map[string]interface{}{}}, nil
	case 32:
		return Swap{Type: SwapStakeDexStakeWrappedSol, Params: map[string]interface{}{}}, nil
	case 33:
		// StakeDexSwapViaStake with bridge_stake_seed
		if offset+4 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for StakeDexSwapViaStake swap")
		}
		bridgeStakeSeed := binary.LittleEndian.Uint32(data[offset : offset+4])
		return Swap{Type: SwapStakeDexSwapViaStake, Params: map[string]interface{}{
			"bridge_stake_seed": bridgeStakeSeed,
		}}, nil
	case 34:
		return Swap{Type: SwapGooseFXV2, Params: map[string]interface{}{}}, nil
	case 35:
		return Swap{Type: SwapPerps, Params: map[string]interface{}{}}, nil
	case 36:
		return Swap{Type: SwapPerpsAddLiquidity, Params: map[string]interface{}{}}, nil
	case 37:
		return Swap{Type: SwapPerpsRemoveLiquidity, Params: map[string]interface{}{}}, nil
	case 38:
		return Swap{Type: SwapMeteoraDlmm, Params: map[string]interface{}{}}, nil
	case 39:
		// OpenBookV2 with side parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for OpenBookV2 swap")
		}
		side := "Bid"
		if data[offset] != 0 {
			side = "Ask"
		}
		return Swap{Type: SwapOpenBookV2, Params: map[string]interface{}{"side": side}}, nil
	case 40:
		return Swap{Type: SwapRaydiumClmmV2, Params: map[string]interface{}{}}, nil
	case 41:
		// StakeDexPrefundWithdrawStake with bridge_stake_seed
		if offset+4 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for StakeDexPrefundWithdrawStake swap")
		}
		bridgeStakeSeed := binary.LittleEndian.Uint32(data[offset : offset+4])
		return Swap{Type: SwapStakeDexPrefundWithdrawStake, Params: map[string]interface{}{
			"bridge_stake_seed": bridgeStakeSeed,
		}}, nil
	case 42:
		// Clone with multiple parameters
		if offset+3 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for Clone swap")
		}
		poolIndex := data[offset]
		quantityIsInput := data[offset+1] != 0
		quantityIsCollateral := data[offset+2] != 0
		return Swap{Type: SwapClone, Params: map[string]interface{}{
			"pool_index":             poolIndex,
			"quantity_is_input":      quantityIsInput,
			"quantity_is_collateral": quantityIsCollateral,
		}}, nil
	case 43:
		// SanctumS with multiple parameters
		if offset+10 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for SanctumS swap")
		}
		srcLstValueCalcAccs := data[offset]
		dstLstValueCalcAccs := data[offset+1]
		srcLstIndex := binary.LittleEndian.Uint32(data[offset+2 : offset+6])
		dstLstIndex := binary.LittleEndian.Uint32(data[offset+6 : offset+10])
		return Swap{Type: SwapSanctumS, Params: map[string]interface{}{
			"src_lst_value_calc_accs": srcLstValueCalcAccs,
			"dst_lst_value_calc_accs": dstLstValueCalcAccs,
			"src_lst_index":           srcLstIndex,
			"dst_lst_index":           dstLstIndex,
		}}, nil
	case 44:
		// SanctumSAddLiquidity with parameters
		if offset+5 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for SanctumSAddLiquidity swap")
		}
		lstValueCalcAccs := data[offset]
		lstIndex := binary.LittleEndian.Uint32(data[offset+1 : offset+5])
		return Swap{Type: SwapSanctumSAddLiquidity, Params: map[string]interface{}{
			"lst_value_calc_accs": lstValueCalcAccs,
			"lst_index":           lstIndex,
		}}, nil
	case 45:
		// SanctumSRemoveLiquidity with parameters
		if offset+5 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for SanctumSRemoveLiquidity swap")
		}
		lstValueCalcAccs := data[offset]
		lstIndex := binary.LittleEndian.Uint32(data[offset+1 : offset+5])
		return Swap{Type: SwapSanctumSRemoveLiquidity, Params: map[string]interface{}{
			"lst_value_calc_accs": lstValueCalcAccs,
			"lst_index":           lstIndex,
		}}, nil
	case 46:
		return Swap{Type: SwapRaydiumCP, Params: map[string]interface{}{}}, nil
	case 47:
		// WhirlpoolSwapV2 with a_to_b and remaining_accounts_info
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for WhirlpoolSwapV2 swap")
		}
		aToB := data[offset] != 0
		// remaining_accounts_info is optional and complex, skip for now
		return Swap{Type: SwapWhirlpoolSwapV2, Params: map[string]interface{}{
			"a_to_b": aToB,
		}}, nil
	case 48:
		return Swap{Type: SwapOneIntro, Params: map[string]interface{}{}}, nil
	case 49:
		return Swap{Type: SwapPumpdotfunWrappedBuy, Params: map[string]interface{}{}}, nil
	case 50:
		return Swap{Type: SwapPumpdotfunWrappedSell, Params: map[string]interface{}{}}, nil
	case 51:
		return Swap{Type: SwapPerpsV2, Params: map[string]interface{}{}}, nil
	case 52:
		return Swap{Type: SwapPerpsV2AddLiquidity, Params: map[string]interface{}{}}, nil
	case 53:
		return Swap{Type: SwapPerpsV2RemoveLiquidity, Params: map[string]interface{}{}}, nil
	case 54:
		return Swap{Type: SwapMoonshotWrappedBuy, Params: map[string]interface{}{}}, nil
	case 55:
		return Swap{Type: SwapMoonshotWrappedSell, Params: map[string]interface{}{}}, nil
	case 56:
		return Swap{Type: SwapStabbleStableSwap, Params: map[string]interface{}{}}, nil
	case 57:
		return Swap{Type: SwapStabbleWeightedSwap, Params: map[string]interface{}{}}, nil
	case 58:
		// Obric with x_to_y parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for Obric swap")
		}
		xToY := data[offset] != 0
		return Swap{Type: SwapObric, Params: map[string]interface{}{"x_to_y": xToY}}, nil
	case 59:
		return Swap{Type: SwapFoxBuyFromEstimatedCost, Params: map[string]interface{}{}}, nil
	case 60:
		// FoxClaimPartial with is_y parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for FoxClaimPartial swap")
		}
		isY := data[offset] != 0
		return Swap{Type: SwapFoxClaimPartial, Params: map[string]interface{}{"is_y": isY}}, nil
	case 61:
		// SolFi with is_quote_to_base parameter
		if offset+1 > len(data) {
			return Swap{}, fmt.Errorf("not enough data for SolFi swap")
		}
		isQuoteToBase := data[offset] != 0
		return Swap{Type: SwapSolFi, Params: map[string]interface{}{"is_quote_to_base": isQuoteToBase}}, nil
	case 76:
		return Swap{Type: Woofi, Params: map[string]interface{}{}}, nil
	case 108:
		return Swap{Type: SwapPumpdotfunAmmBuy, Params: map[string]interface{}{}}, nil
	case 109:
		return Swap{Type: SwapPumpdotfunAmmSell, Params: map[string]interface{}{}}, nil
	default:
		return Swap{Type: SwapType(fmt.Sprintf("Unknown_%d", swapTypeIndex)), Params: map[string]interface{}{}}, nil
	}
}

// updateOffsetForSwapType 根据交换类型更新偏移量
func updateOffsetForSwapType(swapTypeIndex uint8, offset int) int {
	switch swapTypeIndex {
	case 8, 12, 15, 16, 17, 18, 21, 23, 24, 27, 28, 39, 47, 58, 60, 61: // 有1字节参数的类型
		return offset + 1
	case 29: // Symmetry有16字节参数
		return offset + 16
	case 33, 41: // 有4字节参数的类型
		return offset + 4
	case 42: // Clone有3字节参数
		return offset + 3
	case 43: // SanctumS有10字节参数
		return offset + 10
	case 44, 45: // SanctumS Add/Remove Liquidity有5字节参数
		return offset + 5
	default:
		return offset // 没有参数
	}
}

// printJupiterV6Results 打印详细的解析结果
func printJupiterV6Results(params *JupiterSwapParams) {
	fmt.Println("\n=== Jupiter V6 Instruction Analysis ===")
	fmt.Printf("Instruction Type: %s\n", params.InstructionType)

	if params.ID != 0 {
		fmt.Printf("ID: %d\n", params.ID)
	}

	fmt.Printf("\nRoute Plan (%d steps):\n", len(params.RoutePlan))
	for i, step := range params.RoutePlan {
		fmt.Printf("  Step %d:\n", i+1)
		fmt.Printf("    Swap: %s\n", step.Swap.Type)
		if len(step.Swap.Params) > 0 {
			fmt.Printf("    Parameters: %v\n", step.Swap.Params)
		}
		fmt.Printf("    Percent: %d%%\n", step.Percent)
		fmt.Printf("    Input Index: %d -> Output Index: %d\n", step.InputIndex, step.OutputIndex)
	}

	fmt.Printf("\nSwap Parameters:\n")
	if params.InAmount != 0 {
		fmt.Printf("  In Amount: %d\n", params.InAmount)
	}
	if params.OutAmount != 0 {
		fmt.Printf("  Out Amount: %d\n", params.OutAmount)
	}
	if params.QuotedOutAmount != 0 {
		fmt.Printf("  Quoted Out Amount: %d\n", params.QuotedOutAmount)
	}
	if params.QuotedInAmount != 0 {
		fmt.Printf("  Quoted In Amount: %d\n", params.QuotedInAmount)
	}
	fmt.Printf("  Slippage BPS: %d (%.2f%%)\n", params.SlippageBps, float64(params.SlippageBps)/100.0)
	fmt.Printf("  Platform Fee BPS: %d (%.2f%%)\n", params.PlatformFeeBps, float64(params.PlatformFeeBps)/100.0)
	if params.MinAmountOut != 0 {
		fmt.Printf("  Min Amount Out: %d\n", params.MinAmountOut)
	}

	// 使用6位小数显示代币数量
	fmt.Printf("\nFormatted Values (6 decimals):\n")
	if params.InAmount != 0 {
		fmt.Printf("  In Amount: %.6f\n", float64(params.InAmount)/1000000.0)
	}
	if params.OutAmount != 0 {
		fmt.Printf("  Out Amount: %.6f\n", float64(params.OutAmount)/1000000.0)
	}
	if params.QuotedOutAmount != 0 {
		fmt.Printf("  Quoted Out Amount: %.6f\n", float64(params.QuotedOutAmount)/1000000.0)
	}
	if params.QuotedInAmount != 0 {
		fmt.Printf("  Quoted In Amount: %.6f\n", float64(params.QuotedInAmount)/1000000.0)
	}
	if params.MinAmountOut != 0 {
		fmt.Printf("  Min Amount Out: %.6f\n", float64(params.MinAmountOut)/1000000.0)
	}

	// 生成 JSON 格式的输出
	fmt.Printf("\nJSON Format:\n")
	printJSONFormat(params)
}

// printJSONFormat 打印JSON格式的输出
func printJSONFormat(params *JupiterSwapParams) {
	fmt.Printf("{\n")
	fmt.Printf("  \"instruction_type\": \"%s\",\n", params.InstructionType)
	if params.ID != 0 {
		fmt.Printf("  \"id\": %d,\n", params.ID)
	}
	fmt.Printf("  \"route_plan\": [\n")
	for i, step := range params.RoutePlan {
		fmt.Printf("    {\n")
		fmt.Printf("      \"swap\": {\"%s\": %s},\n", step.Swap.Type, formatParams(step.Swap.Params))
		fmt.Printf("      \"percent\": %d,\n", step.Percent)
		fmt.Printf("      \"input_index\": %d,\n", step.InputIndex)
		fmt.Printf("      \"output_index\": %d\n", step.OutputIndex)
		if i < len(params.RoutePlan)-1 {
			fmt.Printf("    },\n")
		} else {
			fmt.Printf("    }\n")
		}
	}
	fmt.Printf("  ],\n")
	if params.InAmount != 0 {
		fmt.Printf("  \"in_amount\": \"%d\",\n", params.InAmount)
	}
	if params.OutAmount != 0 {
		fmt.Printf("  \"out_amount\": \"%d\",\n", params.OutAmount)
	}
	if params.QuotedOutAmount != 0 {
		fmt.Printf("  \"quoted_out_amount\": \"%d\",\n", params.QuotedOutAmount)
	}
	if params.QuotedInAmount != 0 {
		fmt.Printf("  \"quoted_in_amount\": \"%d\",\n", params.QuotedInAmount)
	}
	fmt.Printf("  \"slippage_bps\": \"%d\",\n", params.SlippageBps)
	fmt.Printf("  \"platform_fee_bps\": %d\n", params.PlatformFeeBps)
	fmt.Printf("}\n")
}

// formatParams 格式化参数为 JSON 字符串
func formatParams(params map[string]interface{}) string {
	if len(params) == 0 {
		return "{}"
	}

	var parts []string
	for k, v := range params {
		switch val := v.(type) {
		case bool:
			parts = append(parts, fmt.Sprintf("\"%s\": %t", k, val))
		case string:
			parts = append(parts, fmt.Sprintf("\"%s\": \"%s\"", k, val))
		case uint32:
			parts = append(parts, fmt.Sprintf("\"%s\": %d", k, val))
		case uint64:
			parts = append(parts, fmt.Sprintf("\"%s\": %d", k, val))
		default:
			parts = append(parts, fmt.Sprintf("\"%s\": %v", k, val))
		}
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// bytesEqual 比较两个字节数组是否相等
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// 保留原有的辅助函数
func resolveAddressLookupTables(tx *solana.Transaction, rpcClient *rpc.Client) error {
	if !tx.Message.IsVersioned() {
		return nil // Not a versioned transaction
	}

	lookups := tx.Message.GetAddressTableLookups()
	if lookups == nil || lookups.NumLookups() == 0 {
		return nil // No lookups to resolve
	}

	tableIDs := lookups.GetTableIDs()
	//fmt.Printf("Found %d lookup tables\n", len(tableIDs))

	resolutions := make(map[solana.PublicKey]solana.PublicKeySlice)
	for _, tableID := range tableIDs {
		fmt.Printf("Fetching lookup table: %s\n", tableID.String())

		info, err := rpcClient.GetAccountInfo(
			context.Background(),
			tableID,
		)
		if err != nil {
			return fmt.Errorf("error fetching lookup table: %v", err)
		}

		tableContent, err := lookup.DecodeAddressLookupTableState(info.GetBinary())
		if err != nil {
			return fmt.Errorf("error decoding lookup table: %v", err)
		}

		resolutions[tableID] = tableContent.Addresses
		fmt.Printf("Resolved %d addresses from lookup table\n", len(tableContent.Addresses))
	}

	// Set the address tables
	err := tx.Message.SetAddressTables(resolutions)
	if err != nil {
		return fmt.Errorf("error setting address tables: %v", err)
	}

	// Resolve lookups
	err = tx.Message.ResolveLookups()
	if err != nil {
		return fmt.Errorf("error resolving lookups: %v", err)
	}

	fmt.Println("Successfully resolved address lookups!")
	return nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 添加用于测试的独立函数
func testJupiterParsing() {
	fmt.Println("\n=== Testing Jupiter V6 Parsing ===")

	// 测试案例 1: SharedAccountsRoute
	fmt.Println("\n--- Test Case 1: SharedAccountsRoute ---")
	hexData1 := "C1209B3341D69C810004000000075F0002110005000211016402033D006403042626F600040000005D61040D00000000640000"
	data1, _ := hex.DecodeString(hexData1)

	result1, err := parseJupiterV6Instruction(data1)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		printJupiterV6Results(result1)

		// 验证结果
		fmt.Println("\n--- Validation ---")
		fmt.Printf("Expected vs Actual:\n")
		fmt.Printf("  ID: 0 vs %d %s\n", result1.ID, boolToCheckmark(result1.ID == 0))
		fmt.Printf("  Route steps: 4 vs %d %s\n", len(result1.RoutePlan), boolToCheckmark(len(result1.RoutePlan) == 4))
	}

	// 测试案例 2: Route instruction
	fmt.Println("\n--- Test Case 2: Route ---")
	hexData2 := "E517CB977AE3AD2A0100000048640001497ECC010000000046F5828E04000000AC0355"
	data2, _ := hex.DecodeString(hexData2)

	result2, err := parseJupiterV6Instruction(data2)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		printJupiterV6Results(result2)

		// 验证结果
		fmt.Println("\n--- Validation ---")
		fmt.Printf("Expected vs Actual:\n")
		fmt.Printf("  Route steps: 1 vs %d %s\n", len(result2.RoutePlan), boolToCheckmark(len(result2.RoutePlan) == 1))
	}
}

// boolToCheckmark 将布尔值转换为检查标记
func boolToCheckmark(b bool) string {
	if b {
		return "✓"
	}
	return "✗"
}
