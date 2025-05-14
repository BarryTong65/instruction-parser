# Jupiter 交易解析流程

本文档描述了 Jupiter 交易解析的完整流程，包括指令解析和事件解析两个主要部分。

## 流程图

```mermaid
graph TD
%% 主流程
A[获取交易签名] --> B[通过 RPC 获取交易]
B --> C[解析交易]
C --> D{是否为版本化交易?}
D -->|是| E[解析地址查找表]
D -->|否| F[直接处理]
E --> F
F --> G[遍历交易指令]
G --> H{是否为 Jupiter 程序?}
H -->|是| I[开始指令解析]
H -->|否| J[检查下一条指令]
J --> G

%% Route 解析分支
I --> K[读取前8字节 Discriminator]
K --> L{匹配指令类型}
L -->|route| M[parseRouteInstruction]
L -->|sharedAccountsRoute| N[parseSharedAccountsRoute]
L -->|exactOutRoute| O[parseExactOutRoute]
L -->|其他| P[返回错误]

%% Route 解析详细流程
M --> M1[跳过前8字节判别码]
M1 --> M2[读取4字节 Route Plan Count]
M2 --> M3[循环解析每个 Route Plan Step]
M3 --> M4[解析1字节 Swap Type Index]
M4 --> M5[根据 Index 解码 Swap Type]
M5 --> M6{Swap Type 是否有参数?}
M6 -->|有| M7[读取对应长度的参数]
M6 -->|无| M8[跳过]
M7 --> M8
M8 --> M9[读取1字节 Percent]
M9 --> M10[读取1字节 Input Index]
M10 --> M11[读取1字节 Output Index]
M11 --> M12{是否有更多 Step?}
M12 -->|是| M3
M12 -->|否| M13[读取8字节 In Amount]
M13 --> M14[读取8字节 Quoted Out Amount]
M14 --> M15[读取2字节 Slippage BPS]
M15 --> M16[读取1字节 Platform Fee BPS]
M16 --> M17[计算 Min Amount Out]
M17 --> M18[构建 JupiterSwapParams 对象]

%% SharedAccounts 解析
N --> N1[跳过前8字节判别码]
N1 --> N2[读取1字节 ID]
N2 --> N3[读取4字节 Route Plan Count]
N3 --> N4[解析 Route Plan Steps]
N4 --> N5{指令类型是否为 exactOut?}
N5 -->|是| N6[先读 Out Amount 再读 In Amount]
N5 -->|否| N7[先读 In Amount 再读 Out Amount]
N6 --> N8[读取 Slippage 和 Platform Fee]
N7 --> N8
N8 --> N9[构建 JupiterSwapParams 对象]

%% Event 解析分支
B --> Q[检查交易 Meta]
Q --> R{是否有 InnerInstructions?}
R -->|是| S[遍历内部指令]
R -->|否| T[检查日志]
S --> U{是否为 Jupiter 程序?}
U -->|是| V[检查指令数据]
U -->|否| W[检查下一条内部指令]
V --> X{数据长度是否为128字节?}
X -->|是| Y[检查前8字节是否为 Swap Event Discriminator]
X -->|否| W
Y -->|是| Z[解析 Swap Event]
Y -->|否| W
W --> S

%% Event 解析详细流程
Z --> Z1[读取前8字节 Discriminator]
Z1 --> Z2[读取8-15字节 Unknown Field]
Z2 --> Z3[读取16-47字节 AMM Public Key]
Z3 --> Z4[读取48-79字节 Input Mint]
Z4 --> Z5[读取80-87字节 Input Amount]
Z5 --> Z6[读取88-119字节 Output Mint]
Z6 --> Z7[读取120-127字节 Output Amount]
Z7 --> Z8[构建 SwapEvent 对象]

%% 日志解析
T --> T1[遍历日志消息]
T1 --> T2{是否包含 'Program data:'?}
T2 -->|是| T3[提取 Base58 数据]
T2 -->|否| T4[检查下一条日志]
T3 --> T5[尝试解析为 Swap Event]
T5 --> T6{解析成功?}
T6 -->|是| T7[添加到事件列表]
T6 -->|否| T4
T7 --> T4
T4 --> T1

%% 最终分析
M18 --> AA[收集所有指令]
N9 --> AA
Z8 --> BB[收集所有事件]
T7 --> BB
AA --> CC[生成交换摘要]
BB --> CC
CC --> DD[计算输入/输出代币]
DD --> EE[计算总输入/输出金额]
EE --> FF[构建路由字符串]
FF --> GG[生成完整分析结果]

%% 样式定义
classDef instruction fill:#e1f5fe,stroke:#01579b,stroke-width:2px
classDef event fill:#f3e5f5,stroke:#4a148c,stroke-width:2px
classDef decision fill:#fff3e0,stroke:#e65100,stroke-width:2px
classDef result fill:#e8f5e9,stroke:#1b5e20,stroke-width:2px

class M,M1,M2,M3,M4,M5,M6,M7,M8,M9,M10,M11,M12,M13,M14,M15,M16,M17,M18,N,N1,N2,N3,N4,N5,N6,N7,N8,N9 instruction
class Z,Z1,Z2,Z3,Z4,Z5,Z6,Z7,Z8,T,T1,T2,T3,T4,T5,T6,T7 event
class D,H,L,M6,M12,N5,R,U,X,Y,T2,T6 decision
class GG result
```

## 流程说明

### 主要组件

1. **指令解析**（蓝色节点）
- 处理不同类型的 Jupiter 指令
- 包括 route、sharedAccountsRoute 和 exactOutRoute

2. **事件解析**（紫色节点）
- 从内部指令和日志中提取 Swap Event
- 解析交易执行后的实际结果

3. **决策点**（橙色节点）
- 各种条件判断和分支逻辑

4. **最终结果**（绿色节点）
- 生成完整的交易分析结果

### 关键流程

1. **交易获取与解析**：从 RPC 获取交易数据，处理版本化交易
2. **指令解析**：识别 Jupiter 程序指令并解析其参数
3. **事件解析**：从内部指令和日志中提取 Swap Event
4. **结果汇总**：整合指令和事件数据，生成最终分析结果

### 文件建议

这个文档可以保存为：
- `docs/jupiter-transaction-parsing.md` - 包含完整说明
- `docs/api/jupiter-parsing-flow.md` - API 文档的一部分
- `README.md` 的一个章节 - 如果是项目的核心功能