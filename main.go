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

// Jupiter V6 Event Discriminator (第一个事件的前 8 字节)
var SwapEventDiscriminator = []byte{0xe4, 0x45, 0xa5, 0x2e, 0x51, 0xcb, 0x9a, 0x1d}

// SwapEvent 表示 Jupiter V6 的交换事件
type SwapEvent struct {
	Discriminator []byte           `json:"discriminator"`
	Unknown       []byte           `json:"unknown"`       // 8-15 字节，未知字段
	AMM           solana.PublicKey `json:"amm"`           // 16-47 字节，AMM 程序地址
	InputMint     solana.PublicKey `json:"input_mint"`    // 48-79 字节，输入代币地址
	InputAmount   uint64           `json:"input_amount"`  // 80-87 字节，输入金额
	OutputMint    solana.PublicKey `json:"output_mint"`   // 88-119 字节，输出代币地址
	OutputAmount  uint64           `json:"output_amount"` // 120-127 字节，输出金额
}

// JupiterV6Analysis 表示完整的 Jupiter V6 交易分析结果
type JupiterV6Analysis struct {
	Instructions []JupiterSwapParams `json:"instructions"`
	Events       []SwapEvent         `json:"events"`
	Summary      SwapSummary         `json:"summary"`
}

// SwapSummary 表示交换摘要信息
type SwapSummary struct {
	TotalSwaps  int    `json:"total_swaps"`
	InputToken  string `json:"input_token"`
	OutputToken string `json:"output_token"`
	TotalInput  uint64 `json:"total_input"`
	TotalOutput uint64 `json:"total_output"`
	Route       string `json:"route"`
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

// parseJupiterSwapEvent 解析 Jupiter V6 Swap Event
func parseJupiterSwapEvent(data []byte) (*SwapEvent, error) {
	if len(data) < 128 {
		return nil, fmt.Errorf("swap event data too short: %d bytes", len(data))
	}

	// 检查 discriminator
	if !bytesEqual(data[:8], SwapEventDiscriminator) {
		return nil, fmt.Errorf("invalid swap event discriminator")
	}

	event := &SwapEvent{
		Discriminator: data[:8],
		Unknown:       data[8:16],
	}

	// 解析 AMM 地址 (16-47 字节)
	event.AMM = solana.PublicKeyFromBytes(data[16:48])

	// 解析 Input Mint (48-79 字节)
	event.InputMint = solana.PublicKeyFromBytes(data[48:80])

	// 解析 Input Amount (80-87 字节，小端序)
	event.InputAmount = binary.LittleEndian.Uint64(data[80:88])

	// 解析 Output Mint (88-119 字节)
	event.OutputMint = solana.PublicKeyFromBytes(data[88:120])

	// 解析 Output Amount (120-127 字节，小端序)
	event.OutputAmount = binary.LittleEndian.Uint64(data[120:128])

	return event, nil
}

// parseJupiterSwapEventFromBase58 从 base58 字符串解析 Swap Event
func parseJupiterSwapEventFromBase58(base58Data string) (*SwapEvent, error) {
	data := []byte(base58Data)

	return parseJupiterSwapEvent(data)
}

// extractJupiterEvents 从交易的内部指令中提取 Jupiter 事件
func extractJupiterEvents(tx *rpc.GetTransactionResult) ([]SwapEvent, error) {
	var events []SwapEvent

	if tx.Meta == nil || tx.Meta.InnerInstructions == nil {
		return events, nil
	}

	parseTx, err := tx.Transaction.GetTransaction()
	if err != nil {
		return events, nil
	}

	// 遍历所有内部指令
	for _, innerInst := range tx.Meta.InnerInstructions {
		for _, inst := range innerInst.Instructions {
			// 检查是否是 Jupiter 程序指令
			if inst.ProgramIDIndex < uint16(len(parseTx.Message.AccountKeys)) {
				programID := parseTx.Message.AccountKeys[inst.ProgramIDIndex]
				if programID.Equals(jupiterV6ProgramID) {
					// 尝试解析为 Swap Event
					if len(inst.Data) == 128 {
						// 将 base58 编码的数据转换为字节
						data := []byte(inst.Data)

						// 检查是否是 Swap Event
						if bytesEqual(data[:8], SwapEventDiscriminator) {
							event, err := parseJupiterSwapEvent(data)
							if err == nil {
								events = append(events, *event)
							}
						}
					}
				}
			}
		}
	}

	// 也检查日志中的事件数据
	if tx.Meta.LogMessages != nil {
		for _, logMsg := range tx.Meta.LogMessages {
			// 检查是否是程序数据日志
			if strings.Contains(logMsg, "Program data: ") {
				// 提取数据部分
				parts := strings.Split(logMsg, "Program data: ")
				if len(parts) > 1 {
					base58Data := strings.TrimSpace(parts[1])

					// 尝试解析为 Swap Event
					event, err := parseJupiterSwapEventFromBase58(base58Data)
					if err == nil {
						events = append(events, *event)
					}
				}
			}
		}
	}

	return events, nil
}

// analyzeJupiterV6Transaction 完整分析 Jupiter V6 交易
func analyzeJupiterV6Transaction(tx *rpc.GetTransactionResult, parsedTx *solana.Transaction) (*JupiterV6Analysis, error) {
	analysis := &JupiterV6Analysis{
		Instructions: []JupiterSwapParams{},
		Events:       []SwapEvent{},
	}

	// 1. 解析指令
	for i, inst := range parsedTx.Message.Instructions {
		programIDIndex := int(inst.ProgramIDIndex)
		if programIDIndex >= len(parsedTx.Message.AccountKeys) {
			continue
		}

		programID := parsedTx.Message.AccountKeys[programIDIndex]
		if programID.Equals(jupiterV6ProgramID) {
			fmt.Printf("\nAnalyzing Jupiter instruction at index %d\n", i)

			// 解析指令
			result, err := parseJupiterV6Instruction(inst.Data)
			if err != nil {
				fmt.Printf("Error parsing instruction: %v\n", err)
				continue
			}

			analysis.Instructions = append(analysis.Instructions, *result)
		}
	}

	// 2. 提取事件
	events, err := extractJupiterEvents(tx)
	if err != nil {
		return nil, fmt.Errorf("error extracting events: %v", err)
	}
	analysis.Events = events

	// 3. 生成摘要
	analysis.Summary = generateSwapSummary(analysis.Instructions, analysis.Events)

	return analysis, nil
}

// generateSwapSummary 生成交换摘要
func generateSwapSummary(instructions []JupiterSwapParams, events []SwapEvent) SwapSummary {
	summary := SwapSummary{
		TotalSwaps: len(events),
	}

	if len(events) > 0 {
		// 输入代币是第一个事件的输入
		summary.InputToken = events[0].InputMint.String()
		summary.TotalInput = events[0].InputAmount

		// 输出代币是最后一个事件的输出
		lastEvent := events[len(events)-1]
		summary.OutputToken = lastEvent.OutputMint.String()
		summary.TotalOutput = lastEvent.OutputAmount

		// 构建路由信息
		route := []string{summary.InputToken}
		for _, event := range events {
			route = append(route, event.OutputMint.String())
		}
		summary.Route = strings.Join(route, " -> ")
	}

	return summary
}

// printSwapEvent 打印 Swap Event 的详细信息
func printSwapEvent(event SwapEvent, index int) {
	fmt.Printf("\n=== Swap Event %d ===\n", index+1)
	fmt.Printf("Discriminator: %X\n", event.Discriminator)
	fmt.Printf("Unknown Field: %X\n", event.Unknown)
	fmt.Printf("AMM: %s\n", event.AMM.String())
	fmt.Printf("Input Mint: %s\n", event.InputMint.String())
	fmt.Printf("Input Amount: %d\n", event.InputAmount)
	fmt.Printf("Output Mint: %s\n", event.OutputMint.String())
	fmt.Printf("Output Amount: %d\n", event.OutputAmount)

	// 格式化为 6 位小数
	fmt.Printf("\nFormatted Values (6 decimals):\n")
	fmt.Printf("  Input Amount: %.6f\n", float64(event.InputAmount)/1000000.0)
	fmt.Printf("  Output Amount: %.6f\n", float64(event.OutputAmount)/1000000.0)
}

// printJupiterV6Analysis 打印完整的 Jupiter V6 分析结果
func printJupiterV6Analysis(analysis *JupiterV6Analysis) {
	fmt.Println("\n=== Jupiter V6 Transaction Analysis ===")

	// 打印摘要
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Total Swaps: %d\n", analysis.Summary.TotalSwaps)
	fmt.Printf("  Input Token: %s\n", analysis.Summary.InputToken)
	fmt.Printf("  Output Token: %s\n", analysis.Summary.OutputToken)
	fmt.Printf("  Total Input: %d (%.6f)\n", analysis.Summary.TotalInput, float64(analysis.Summary.TotalInput)/1000000.0)
	fmt.Printf("  Total Output: %d (%.6f)\n", analysis.Summary.TotalOutput, float64(analysis.Summary.TotalOutput)/1000000.0)
	fmt.Printf("  Route: %s\n", analysis.Summary.Route)

	// 打印指令详情
	fmt.Printf("\nInstructions (%d):\n", len(analysis.Instructions))
	for i, inst := range analysis.Instructions {
		fmt.Printf("\n--- Instruction %d ---\n", i+1)
		printJupiterV6Results(&inst)
	}

	// 打印事件详情
	fmt.Printf("\nSwap Events (%d):\n", len(analysis.Events))
	for i, event := range analysis.Events {
		printSwapEvent(event, i)
	}

	// 生成 JSON 输出
	fmt.Printf("\n=== JSON Output ===\n")
	printJupiterV6AnalysisJSON(analysis)
}

// printJupiterV6AnalysisJSON 打印 JSON 格式的分析结果
func printJupiterV6AnalysisJSON(analysis *JupiterV6Analysis) {
	fmt.Printf("{\n")
	fmt.Printf("  \"summary\": {\n")
	fmt.Printf("    \"total_swaps\": %d,\n", analysis.Summary.TotalSwaps)
	fmt.Printf("    \"input_token\": \"%s\",\n", analysis.Summary.InputToken)
	fmt.Printf("    \"output_token\": \"%s\",\n", analysis.Summary.OutputToken)
	fmt.Printf("    \"total_input\": \"%d\",\n", analysis.Summary.TotalInput)
	fmt.Printf("    \"total_output\": \"%d\",\n", analysis.Summary.TotalOutput)
	fmt.Printf("    \"route\": \"%s\"\n", analysis.Summary.Route)
	fmt.Printf("  },\n")

	fmt.Printf("  \"instructions\": [\n")
	for i, inst := range analysis.Instructions {
		fmt.Printf("    {\n")
		fmt.Printf("      \"instruction_type\": \"%s\",\n", inst.InstructionType)
		if inst.ID != 0 {
			fmt.Printf("      \"id\": %d,\n", inst.ID)
		}
		fmt.Printf("      \"in_amount\": \"%d\",\n", inst.InAmount)
		fmt.Printf("      \"quoted_out_amount\": \"%d\",\n", inst.QuotedOutAmount)
		fmt.Printf("      \"slippage_bps\": \"%d\",\n", inst.SlippageBps)
		fmt.Printf("      \"platform_fee_bps\": %d\n", inst.PlatformFeeBps)
		if i < len(analysis.Instructions)-1 {
			fmt.Printf("    },\n")
		} else {
			fmt.Printf("    }\n")
		}
	}
	fmt.Printf("  ],\n")

	fmt.Printf("  \"events\": [\n")
	for i, event := range analysis.Events {
		fmt.Printf("    {\n")
		fmt.Printf("      \"amm\": \"%s\",\n", event.AMM.String())
		fmt.Printf("      \"input_mint\": \"%s\",\n", event.InputMint.String())
		fmt.Printf("      \"input_amount\": \"%d\",\n", event.InputAmount)
		fmt.Printf("      \"output_mint\": \"%s\",\n", event.OutputMint.String())
		fmt.Printf("      \"output_amount\": \"%d\"\n", event.OutputAmount)
		if i < len(analysis.Events)-1 {
			fmt.Printf("    },\n")
		} else {
			fmt.Printf("    }\n")
		}
	}
	fmt.Printf("  ]\n")
	fmt.Printf("}\n")
}

func main() {
	// Transaction signature
	txSignature := solana.MustSignatureFromBase58("5Mckd1q1vKHP7X4r45gcdNoy9gKfjG3jYUG6vyx6tPB3MzKrD44hHiP89PnPGQTV1p6NG56rz1jp6AyxKFtyo4aR")

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

	// 执行完整的 Jupiter V6 分析
	analysis, err := analyzeJupiterV6Transaction(tx, parsedTx)
	if err != nil {
		fmt.Printf("Error analyzing Jupiter V6 transaction: %v\n", err)
		return
	}

	// 打印分析结果
	printJupiterV6Analysis(analysis)
}
