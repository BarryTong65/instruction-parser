# Jupiter V6 Transaction Parser

A Go-based tool for parsing and analyzing Jupiter V6 transactions on the Solana blockchain. This library provides detailed insights into Jupiter V6 swap transactions, including instruction data, route plans, swap events, and more.

## Features

- Parse Jupiter V6 instruction data from Solana transactions
- Decode different instruction types (route, routeWithTokenLedger, sharedAccountsRoute, etc.)
- Extract and analyze swap events from transaction logs and inner instructions
- Support for all major swap protocols in the Jupiter V6 ecosystem
- Handle versioned transactions with address lookup tables
- Generate detailed analysis reports in both human-readable and JSON formats

## Supported Instruction Types

- `route`
- `routeWithTokenLedger`
- `sharedAccountsRoute`
- `sharedAccountsRouteWithTokenLedger`
- `exactOutRoute`
- `sharedAccountsExactOutRoute`

## Supported Swap Protocols

The parser supports over 50 different swap protocols integrated with Jupiter V6, including:

- Saber
- Raydium
- Whirlpool
- Orca
- Openbook
- Phoenix
- And many more (see the `SwapType` constants in the code)

## Usage

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/gagliardetto/solana-go"
    "github.com/gagliardetto/solana-go/rpc"
    "golang.org/x/time/rate"
)

func main() {
    // Transaction signature to analyze
    txSignature := solana.MustSignatureFromBase58("YOUR_TRANSACTION_SIGNATURE")
    
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
    
    // Resolve address lookup tables for versioned transactions
    if parsedTx.Message.IsVersioned() {
        err = resolveAddressLookupTables(parsedTx, rpcClient)
        if err != nil {
            fmt.Printf("Error resolving address lookup tables: %v\n", err)
            return
        }
    }
    
    // Analyze Jupiter V6 transaction
    analysis, err := analyzeJupiterV6Transaction(tx, parsedTx)
    if err != nil {
        fmt.Printf("Error analyzing Jupiter V6 transaction: %v\n", err)
        return
    }
    
    // Print analysis results
    printJupiterV6Analysis(analysis)
}
```

## Example Output

The parser generates detailed information about Jupiter swap transactions, including:

- Transaction summary (total swaps, input/output tokens, amounts)
- Complete route information
- Details of each instruction
- Swap event data with AMM information
- All amounts formatted in both raw and human-readable (6 decimal) format

## Dependencies

- [github.com/gagliardetto/solana-go](https://github.com/gagliardetto/solana-go) - Solana library for Go
- [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate) - Rate limiting functionality

## License

[MIT License](LICENSE) 