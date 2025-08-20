# Signaling proposal to add the DynamicFee module (Resubmission)

This proposal is being resubmitted after the previous attempt did not reach quorum, despite receiving near-unanimous support (99% of voters in favor). The goal of this resubmission is to ensure broader participation and achieve the necessary quorum to move forward with community feedback on this important upgrade.

This signaling proposal aims to gather community feedback on including a new module, the `x/dynamicfee`, in the AtomOne chain. This module implements the Additive Increase Multiplicative Decrease [(AIMD) EIP-1559](https://arxiv.org/abs/2110.04753) mechanism to automatically adjust transactions fees based on the block utilization. This mechanism aims to keep the block utilization at a target using transactions fees as a control variable.

## Motivation

On March 2025, the AtomOne chain suffered from a DOS attack. The attacker performed multiple bloated multi-send transactions aiming at slowing down the AtomOne network and overflowing the capacity of AtomOne’s nodes. Thanks to the swift response of maintainers and validators the attack was interrupted by increasing the transaction fees. 

The DynamicFee module aims at controlling the congestion of the network and at automating the response to DOS attacks, by dynamically adjusting fees pricing based on the network block utilization.

## Implementation

The AIMD EIP-1559 fee pricing is a slight modification to Ethereum's EIP-1559 fee pricing. Specifically it introduces the notion of an adaptive learning rate which scales the base gas price more aggressively when the network is congested and less aggressively when the network is not congested. This is primarily done to address the often cited criticism of EIP-1559 that it's base fee often lags behind the current demand for block space, since the learning rate is a constant value set at 12.5%.

EIP-1559 utilizes the following formula to compute the base fee:

$$
GasPrice_{t} =  GasPrice_{t-1} \\times (1+ 0.125 \\times \\frac {(BlockSize_{t-1} - TargetBlockSize)}{TargetBlockSize}) 
$$
Where:

- $GasPrice_{t}$ is the newly computed gas price for the next block.
- $GasPrice_{t-1}$ is the gas price of the current block.
- $TargetBlockSize$ is the target block size in bytes. This must be a value that is greater than `0`.
- $BlockSize_{t-1}$ is the curent block size.

AIMD EIP-1559 introduces a few new parameters to the EIP-1559 fee pricing. In the implementation of this feature for AtomOne the *amount of bytes* was replaced in favor of *amount of gas* as the unit of consumption of a block. 

The calculation for the updated base fee for the a block is as follows:

$$
AvgBlockUse = \\frac {\\sum_{n=1}^{N} BlockGas_{t-n}}{N \\times MaxBlockGas}
$$

$$
LR_{t} = 
\\begin{cases}
  min(MaxLR,\\alpha + LR_{t-1}) & \\text{if } AvgBlockUse \\le \\gamma  \\\\
  min(MaxLR,\\alpha + LR_{t-1}) & \\text{if } AvgBlockUse \\ge 1-\\gamma \\\\
  max(MinLR,\\beta \\times LR_{t-1}) & \\text{otherwise}
\\end{cases}
$$

$$
GasPrice_{t} =  GasPrice_{t-1} \\times (1+ LR_{t} \\times \\frac {(BlockGas_{t-1} - TargetBlockGas)}{TargetBlockGas}) 
$$

- $\\alpha$ is the amount we use to additively increase the learning rate. This must be a value that is greater than 0.0.
- $\\beta$ is the amount used to multiplicatively decrease the learning rate. This must be a value that is greater than 0.0. 
- $N$ is the number of blocks considered when computing the average block utilization. 
- $\\gamma$ determines if the learning rate is additively increased or multiplicatively decreased based on the average block utilization. This must be a value in the $[0,1]$ range.
- $MaxLR$ is the maximum learning rate that can be applied. This must be a value between $[0, 1]$.
- $MinLR$ is the minimum learning rate that can be applied. This must be a value between $[0, 1]$.
- $BlockGas_{t}$ is the amount of gas of the block at time $t$.
- $MaxBlockGas$ is the maximum amount of gas of a block.
- $TargetBlockGas$ is the targeted amount of gas of a block.

When the current block gas is close to the `TargetBlockGas` (in other words, when `AvgBlockUse` is in the `gamma` range), then the base gas price is close to the right value, so the algorithm reduces the learning rate to reduce the size of oscillations. By contrast, if the current block gas is too small or too high (`AvgBlockUse` is out of `gamma` range), then the base fee is apparently far away from its equilibrium value, and the algorithm increases the learning rate.

## Key Module Elements

### Parameters

The DynamicFee module contains the following parameters:

- `alpha`, the amount used to additively increase the learning rate.

- `beta`, the amount used to multiplicatively decrease the learning rate.

- `gamma`, determines the thresholds used to update the learning rate.

- `min_gas_price`, determines the initial gas price of the module and the global minimum for the network.

- `target_block_utilization`, the target block utilization expressed as a decimal between 0 and 1.

- `min_learning_rate`, the minimum value of the learning rate.

- `max_learning_rate`, the maximum value of the learning rate.

- `window`, the window size used to compute the average block utilization.

- `fee_denom`, the denom that is used for the fee payments.

- `enabled`, a boolean that determines if the DynamicFee module is enabled.

### State

The `x/dynamicfee` module keeps state of the following primary objects:

1. Current base-fee.

2. Current learning rate.

3. Moving window of block size.

### Query

A user can interact with the DynamicFee module using the CLI or using gRPC endpoints.

### Testing and Testnet

The DynamicFee module has been rigorously tested via unit and end-to-end tests.

The AtomOne public testnet will undergo a coordinated upgrade that will include the DynamicFee module before a software upgrade is proposed on mainnet, allowing more testing before mainnet deployment.

## Potential risks

### User Experience Challenges

Users will experience fluctuations in the fees required for a transaction that are dependent on the current state of the network. 

### Clients Integration

Clients will need to adapt, as this is a non standard cosmos fee mechanism, and query the chain to request current gas prices.

## Audit

An audit covering the entire AtomOne codebase and the `x/dynamicfee` module is currently being performed.

## Upgrade Process

The integration of the `x/dynamicfee` module is contingent upon the successful completion of a third-party audit and thorough validation of its functionality. An upgrade proposal including the dynamic quorum will be submitted for governance voting should this signaling proposal pass.

## Codebase

[https://github.com/atomone-hub/atomone/blob/main/x/dynamicfee/README.md](https://github.com/atomone-hub/atomone/blob/main/x/dynamicfee/README.md)

[https://github.com/atomone-hub/atomone/blob/main/x/dynamicfee/AIMD.md](https://github.com/atomone-hub/atomone/blob/main/x/dynamicfee/AIMD.md)

[https://github.com/atomone-hub/atomone/pull/114](https://github.com/atomone-hub/atomone/pull/114/)

[https://github.com/atomone-hub/atomone/pull/170](https://github.com/atomone-hub/atomone/pull/170)

## Voting options

- Yes: You are in favor of adding the DynamicFee module.

- No: You are against having the DynamicFee module.

- ABSTAIN - You wish to contribute to the quorum but you formally decline to vote either for or against the proposal.

