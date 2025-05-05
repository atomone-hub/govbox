# Signaling proposal for new model to make proposal deposits dynamic

This signaling proposal aims to gather community feedback on updating the `x/gov` module to implement a dynamic proposal deposit mechanism. This feature would replace the current static `MinDeposit` and `MinInitialDeposit` values with an adaptive system that automatically adjusts deposits based on governance activity. If approved, this feature will be included in a future AtomOne software upgrade.

The current proposal presents a revision to proposal [#6](https://gov.atom.one/proposals/6). The model in proposal #6 was changed to remove time-based increases and deactivation decreases. The system now increases `MinDeposit` and `MinInitialDeposit`, respectively, upon proposal activation or submission - once the target is exceeded - and decreases these two values with time when below the respective targets.

# Motivation

### Addressing Governance Spam

Many Cosmos-based chains suffer from governance spam, where users submit low-cost proposals containing misleading information or scams. While frontend filtering and initial deposit requirements exist, dynamically adjusting `MinInitialDeposit` in response to governance activity can provide a more robust solution.

### Preventing Proposal Overload

An excessive number of active governance proposals can overwhelm stakers, leading to reduced voter participation and governance inefficiencies. The proposed mechanism ensures that governance remains focused by dynamically increasing the deposit requirements when too many proposals are active.

### Reducing Manual Adjustments

Currently, adjusting `MinDeposit` requires governance intervention, making it difficult to respond quickly to changes in proposal volume. A self-regulating deposit mechanism eliminates the need for frequent governance proposals to modify deposit parameters.

# Implementation

The `x/gov` module will be updated to replace the fixed `MinDeposit` and `MinInitialDeposit` parameters with dynamic (independently updated) values determined by the following formula:

$D_{t+1} = \max(D_{\min}, D_t \times (1 + \alpha \times \sigma))$

$\alpha = \begin{cases} \alpha_{up} & n_t \geq N \\-\alpha_{down} & n_t \lt N\end{cases} \\

\sigma = \begin{cases} 1 & n_t \geq N \\\sqrt[k]{| n_t - N |} & n_t \lt N\end{cases}$

$$
k \in {1, 2, 3, ..., 100}\\
0 \lt \alpha_{down} \lt 1\\
0 \lt \alpha_{up} \lt 1\\
$$

Where:

- $D_{t+1}$ is the updated deposit value.
- $D_{\min}$ is the floor deposit value.
- $D_t$ is the current deposit value.
- $n_t$ is the number of active proposals (either in voting period or deposit period)
- $N$ is the target number of active proposals.
- $k$ is a is a sensitivity factor that determines how sharply the deposit decreases in relation to the distance from the target. It must be a positive integer between 1 and 100.
- $\alpha_{up}$ and $\alpha_{down}$ define the rate of increase/decrease and must be between 0 and 1.

The mechanism updates dynamically:

- When proposals enter the voting/deposit period for increases, when the respective targets are met or exceeded.
- At regular time intervals (ticks) for decreases, allowing deposits to gradually decrease even when proposal counts remain stable.

### Key Module Changes

1. **Deprecation of Fixed Deposit Values**
    - `MinDeposit` and `MinInitialDepositRatio` will be deprecated. Attempting to set these parameters in the `x/gov` module will result in an error.
2. **New Dynamic Deposit Parameters:**
    
    The following parameters will be available to fine tune both `MinDeposit` and `MinInitialDeposit`, with each deposit type having their separate collection:
    
    - `floor_value`: Minimum possible deposit requirement.
    - `update_period`: Time interval for decreases of the deposit when below the target.
    - `target_active_proposals`: The ideal number of active proposals the system aims to maintain.
    - `increase_ratio` / `decrease_ratio`: Defines how fast deposits adjust to changes in the number of active proposals.
    - `sensitivity_target_distance`: Controls the steepness of deposit decreases based on difference between  the number of currently active proposals and `target_active_proposals`.

# Testing and Testnet

The dynamic deposit feature has been rigorously tested via unit and end-to-end tests.

The AtomOne public testnet will undergo a coordinated upgrade that will include the dynamic deposit feature before a software upgrade is proposed on mainnet, allowing more testing before mainnet deployment.

# Potential risks

### Increased Complexity

Automatically adjusting deposit requirements adds computational load. In particular, the frequency of time-based decreases should be carefully selected to be sufficient but not too frequent.

### User Experience Challenges

Users may find it harder to predict the amount of deposit required for a proposal. This can be mitigated with clear client-side tools that display real-time deposit requirements estimates.

# Audit

An audit covering the entire AtomOne codebase and the `x/gov` module including the original implementation of the dynamic deposit (proposal [#6](https://gov.atom.one/proposals/6)) started in February 2025 and has been completed in March 2025, with no findings. The audit report is available on the AtomOne blockchain code repository.
The changes proposed by this revision will undergo an additional audit of the incremental difference before a software upgrade that includes the presented system is submitted on the mainnet.

# Upgrade process

The implementation of the revised model of the dynamic deposit is contingent upon the successful completion of a third-party audit and thorough validation of its functionality. Once these conditions are met, we anticipate releasing this feature as part of a future AtomOne upgrade. 

# Codebase

- [https://github.com/atomone-hub/atomone/blob/97b6b989931dbd710cf91f33afd489f292502a9b/docs/architecture/adr-003-governance-proposal-deposit-auto-throttler.md](https://github.com/atomone-hub/atomone/blob/97b6b989931dbd710cf91f33afd489f292502a9b/docs/architecture/adr-003-governance-proposal-deposit-auto-throttler.md)
- [https://github.com/atomone-hub/atomone/pull/69](https://github.com/atomone-hub/atomone/pull/69)
- [https://github.com/atomone-hub/atomone/pull/65](https://github.com/atomone-hub/atomone/pull/65)
- [https://github.com/atomone-hub/atomone/pull/105](https://github.com/atomone-hub/atomone/pull/105)

# Voting options

- Yes: You are in favor of introducing a dynamic deposit for governance proposals.
- No: You are against having a dynamic deposit for governance proposals.
- ABSTAIN - You wish to contribute to the quorum but you formally decline to vote either for or against the proposal.
