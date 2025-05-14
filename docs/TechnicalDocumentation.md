# Jupiter V6 指令解析详细说明

## 概述

这个代码实现了对 Solana 上 Jupiter V6 聚合器的交换指令和事件的解析。Jupiter 是 Solana 上最大的 DEX 聚合器，V6 版本支持多种交换类型和路由策略。

## 1. 数据结构和常量定义

### 1.1 指令判别码 (Discriminators)

```go
var InstructionDiscriminators = map[string][]byte{
"route":                              {0xE5, 0x17, 0xCB, 0x97, 0x7A, 0xE3, 0xAD, 0x2A},
"routeWithTokenLedger":               {0x96, 0x56, 0x47, 0x74, 0xA7, 0x5D, 0x0E, 0x68},
"sharedAccountsRoute":                {0xC1, 0x20, 0x9B, 0x33, 0x41, 0xD6, 0x9C, 0x81},
// ... 更多判别码
}
```

**作用**: 前8字节用于识别指令类型。每种指令都有唯一的判别码，通过比较指令数据的前8字节来确定指令类型。

### 1.2 事件判别码

```go
var SwapEventDiscriminator = []byte{0xe4, 0x45, 0xa5, 0x2e, 0x51, 0xcb, 0x9a, 0x1d}
```

**作用**: 用于识别 Jupiter V6 的交换事件。

## 2. Jupiter 指令解析

### 2.1 指令解析流程

```go
func parseJupiterV6Instruction(data []byte) (*JupiterSwapParams, error) {
if len(data) < 8 {
return nil, fmt.Errorf("instruction data too short")
}

discriminator := data[:8]  // 前8字节是判别码

// 根据判别码调用相应的解析函数
if bytesEqual(discriminator, InstructionDiscriminators["route"]) {
return parseRouteInstruction(data, "route")
}
// ... 其他指令类型检查
}
```

### 2.2 Route 指令解析 (标准路由)

```go
func parseRouteInstruction(data []byte, instructionType string) (*JupiterSwapParams, error) {
offset := 8  // 跳过判别码

// 字节 8-11: 路由计划数量 (4字节，小端序)
routePlanCount := binary.LittleEndian.Uint32(data[offset : offset+4])
offset += 4

// 解析每个路由计划步骤
routePlan := make([]RoutePlanStep, routePlanCount)
for i := uint32(0); i < routePlanCount; i++ {
step, newOffset, err := parseRoutePlanStep(data, offset)
routePlan[i] = step
offset = newOffset
}

// 解析交换参数
inAmount := binary.LittleEndian.Uint64(data[offset : offset+8])      // 8字节
offset += 8
quotedOutAmount := binary.LittleEndian.Uint64(data[offset : offset+8]) // 8字节
offset += 8
slippageBps := binary.LittleEndian.Uint16(data[offset : offset+2])     // 2字节
offset += 2
platformFeeBps := data[offset]                                         // 1字节

return &JupiterSwapParams{...}, nil
}
```

**字节布局**:
- **0-7**: 判别码 (8字节)
- **8-11**: 路由计划数量 (4字节，小端序)
- **12-N**: 路由计划步骤 (每个步骤长度可变)
- **N-N+7**: 输入金额 (8字节，小端序)
- **N+8-N+15**: 预期输出金额 (8字节，小端序)
- **N+16-N+17**: 滑点 BPS (2字节，小端序)
- **N+18**: 平台费用 BPS (1字节)

### 2.3 SharedAccountsRoute 解析 (共享账户路由)

```go
func parseSharedAccountsRoute(data []byte, instructionType string) (*JupiterSwapParams, error) {
offset := 8  // 跳过判别码

// 字节 8: ID (1字节)
id := data[offset]
offset++

// 字节 9-12: 路由计划数量 (4字节，小端序)
routePlanCount := binary.LittleEndian.Uint32(data[offset : offset+4])
offset += 4

// 其余解析类似 route 指令
}
```

**字节布局**:
- **0-7**: 判别码 (8字节)
- **8**: ID (1字节)
- **9-12**: 路由计划数量 (4字节，小端序)
- **13-N**: 路由计划步骤
- **N-N+7**: 输入金额 (8字节)
- **N+8-N+15**: 预期输出金额 (8字节)
- **N+16-N+17**: 滑点 BPS (2字节)
- **N+18**: 平台费用 BPS (1字节)

## 3. Route Plan 解析

### 3.1 Route Plan Step 结构

```go
func parseRoutePlanStep(data []byte, offset int) (RoutePlanStep, int, error) {
// 字节 0: 交换类型索引 (1字节)
swapTypeIndex := data[offset]
offset++

// 根据交换类型解析参数
swap, err := decodeSwapType(swapTypeIndex, data, offset)
offset = updateOffsetForSwapType(swapTypeIndex, offset)

// 字节 N: 百分比 (1字节)
percent := data[offset]
offset++

// 字节 N+1: 输入索引 (1字节)
inputIndex := data[offset]
offset++

// 字节 N+2: 输出索引 (1字节)
outputIndex := data[offset]
offset++

return RoutePlanStep{...}, offset, nil
}
```

**Route Plan Step 字节布局**:
- **0**: 交换类型索引 (1字节)
- **1-N**: 交换类型特定参数 (长度可变)
- **N**: 百分比 (1字节，0-100)
- **N+1**: 输入代币索引 (1字节)
- **N+2**: 输出代币索引 (1字节)

### 3.2 交换类型参数解析

不同的交换类型有不同的参数长度：

```go
func updateOffsetForSwapType(swapTypeIndex uint8, offset int) int {
switch swapTypeIndex {
case 8, 12, 15, 16, 17, 18, 21, 23, 24, 27, 28, 39, 47, 58, 60, 61:
return offset + 1  // 1字节参数 (如 a_to_b, side)
case 29:
return offset + 16  // Symmetry: 16字节 (from_token_id + to_token_id)
case 33, 41:
return offset + 4   // 4字节参数 (bridge_stake_seed)
case 42:
return offset + 3   // Clone: 3字节参数
case 43:
return offset + 10  // SanctumS: 10字节参数
case 44, 45:
return offset + 5   // SanctumS Add/Remove: 5字节参数
default:
return offset       // 无参数
}
}
```

## 4. Swap Event 解析

### 4.1 SwapEvent 结构

```go
type SwapEvent struct {
Discriminator []byte           `json:"discriminator"`  // 0-7字节
Unknown       []byte           `json:"unknown"`        // 8-15字节
AMM           solana.PublicKey `json:"amm"`            // 16-47字节
InputMint     solana.PublicKey `json:"input_mint"`     // 48-79字节
InputAmount   uint64           `json:"input_amount"`   // 80-87字节
OutputMint    solana.PublicKey `json:"output_mint"`    // 88-119字节
OutputAmount  uint64           `json:"output_amount"`  // 120-127字节
}
```

### 4.2 Event 解析函数

```go
func parseJupiterSwapEvent(data []byte) (*SwapEvent, error) {
if len(data) < 128 {
return nil, fmt.Errorf("swap event data too short: %d bytes", len(data))
}

// 检查判别码 (0-7字节)
if !bytesEqual(data[:8], SwapEventDiscriminator) {
return nil, fmt.Errorf("invalid swap event discriminator")
}

event := &SwapEvent{
Discriminator: data[:8],    // 0-7字节: 判别码
Unknown:       data[8:16],  // 8-15字节: 未知字段
}

// 16-47字节: AMM 程序地址 (32字节公钥)
event.AMM = solana.PublicKeyFromBytes(data[16:48])

// 48-79字节: 输入代币地址 (32字节公钥)
event.InputMint = solana.PublicKeyFromBytes(data[48:80])

// 80-87字节: 输入金额 (8字节，小端序)
event.InputAmount = binary.LittleEndian.Uint64(data[80:88])

// 88-119字节: 输出代币地址 (32字节公钥)
event.OutputMint = solana.PublicKeyFromBytes(data[88:120])

// 120-127字节: 输出金额 (8字节，小端序)
event.OutputAmount = binary.LittleEndian.Uint64(data[120:128])

return event, nil
}
```

**SwapEvent 字节布局 (总共128字节)**:
- **0-7**: 事件判别码 (8字节)
- **8-15**: 未知字段 (8字节)
- **16-47**: AMM 程序地址 (32字节公钥)
- **48-79**: 输入代币地址 (32字节公钥)
- **80-87**: 输入金额 (8字节，小端序)
- **88-119**: 输出代币地址 (32字节公钥)
- **120-127**: 输出金额 (8字节，小端序)

## 5. 数据转换和格式化

### 5.1 数值转换

代码使用小端序 (LittleEndian) 读取多字节数值：

```go
// 读取32位整数
routePlanCount := binary.LittleEndian.Uint32(data[offset : offset+4])

// 读取64位整数
inAmount := binary.LittleEndian.Uint64(data[offset : offset+8])

// 读取16位整数
slippageBps := binary.LittleEndian.Uint16(data[offset : offset+2])
```

### 5.2 公钥转换

Solana 公钥是32字节的数组：

```go
event.AMM = solana.PublicKeyFromBytes(data[16:48])
```

### 5.3 格式化输出

代码提供多种输出格式：

1. **详细文本输出**: 包含所有字段的可读格式
2. **JSON 输出**: 结构化的 JSON 格式
3. **格式化数值**: 将代币金额格式化为6位小数

```go
// 格式化为6位小数 (假设代币精度为6)
fmt.Printf("Input Amount: %.6f\n", float64(event.InputAmount)/1000000.0)
```

## 6. 完整的解析流程

1. **识别指令**: 通过前8字节判别码确定指令类型
2. **解析结构**: 根据指令类型解析固定字段和可变长度字段
3. **处理路由计划**: 解析每个步骤的交换类型和参数
4. **提取事件**: 从内部指令或日志中提取 SwapEvent
5. **生成摘要**: 创建包含路由信息的交换摘要

## 7. 关键设计考虑

1. **可变长度数据**: Route plan 的数量和每个步骤的参数长度都是可变的
2. **类型安全**: 使用强类型结构体确保数据完整性
3. **错误处理**: 检查数据长度和格式有效性
4. **扩展性**: 支持新的交换类型和指令格式

这个解析器能够处理 Jupiter V6 的各种复杂指令格式，从简单的单步交换到复杂的多步骤聚合路由。