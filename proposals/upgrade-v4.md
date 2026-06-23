# AtomOne v4 upgrade

The AtomOne v4 release is a major release that will follow the standard
governance process. The on-chain voting period will last 3 weeks.

On governance vote approval, validators will be required to update the AtomOne
chain binary at the halt-height specified in the on-chain proposal.

A full technical reference for the v4 release is published at
[atom.one/blog/atomone-v4-technical-reference](https://atom.one/blog/atomone-v4-technical-reference/)
and is the recommended companion to this proposal.

## Release Binary & Upgrade Resources

IMPORTANT: Note that AtomOne v4.0.0 binary MUST be used.

The release can be found [here](https://github.com/atomone-hub/atomone/releases/tag/v4.0.0).
The changelog can be found [here](https://github.com/atomone-hub/atomone/blob/v4.0.0/CHANGELOG.md#v400).

## Proposed Release Contents

This release introduces the following major new features:

- Adopt the AtomOne SDK (`github.com/atomone-hub/cosmos-sdk`, versioned
  `v0.500.1` and based on Cosmos SDK v0.50) as the canonical home for all
  AtomOne-specific governance, staking and fee-market logic, on a release
  trajectory independent from the Cosmos SDK
- Upgrade to IBC-go v10, and CometBFT v0.38
- Introduce **Governors** (ADR-006): a new class of governance participants
  that allows ATONE stakers to delegate governance voting power independently
  from staking, addressing the participation gap created by the Constitution's
  prohibition on validators inheriting delegator votes
- Add the **`x/coredaos`** module, translating the constitutional Steering DAO
  and Oversight DAO into enforceable on-chain capabilities (proposal
  endorsement, annotation, voting-period extension, and constitutional veto)
- Set validator commission to a network-enforced fixed rate of **5%** and
  introduce the **Nakamoto Bonus** reward mechanism (ADR-004), starting at
  η = 3%, to improve stake decentralization
- Migrate the governance module from the `atomone.gov.v1` Protobuf namespace
  to `cosmos.gov.v1`, with a backward-compatibility wrapper preserving all
  pre-existing `atomone.gov.v1` query and message endpoints throughout the v4
  lifecycle (the wrapper is scheduled for removal in v5)
- Migrate `x/dynamicfee` into the AtomOne SDK, with a fix for the unlimited
  block-gas (`MaxGas = -1`) edge case
- Add the **`10-gno`** IBC light client, enabling future IBC connectivity with
  Gno chains running Tendermint2 and laying the groundwork for the
  Validation-as-a-Service shared-security model
- Backport consensus key rotation

The other changes can be found in the AtomOne binary changelog [here](https://github.com/atomone-hub/atomone/blob/v4.0.0/CHANGELOG.md#v400)
as well as the AtomOne SDK changelog [here](https://github.com/atomone-hub/cosmos-sdk/blob/v0.500.1/CHANGELOG.md#v05000---2026-06-22).

## Schedule

With a three-week voting period, in the event the proposal is supported, it
should pass around Jul 15th. The upgrade will be expected to take place the
following week, at Jul 22th. The specific block halt-height is [9,550,000](https://www.mintscan.io/atomone/block/9550000).

## Testing and Testnets

The v4 release has gone through rigorous testing, including e2e tests and
integration tests. The AtomOne v4 upgrade was also subjected to a security
audit carried out by [Oak Security](https://oaksecurity.io/). The full report
is available at
[github.com/atomone-hub/atomone/tree/main/docs](https://github.com/atomone-hub/atomone/blob/main/docs/v4%20-%20Oak%20Audit%20Report.pdf).

Validators and node operators have joined a [public testnet](https://testnet.explorer.allinbits.services/atomone-testnet-1)
to participate in a test upgrade to a release candidate before the AtomOne
network upgrades to the final release.

## Potential risk factors

Although very extensive testing and simulation will have taken place there
always exists a risk that the AtomOne experiences problems due to potential
bugs or errors from the new features. In the case of serious problems,
validators should stop operating the network immediately. Coordination with
validators will happen in the #validator-private channel of the AtomOne
Community Discord to create and execute a contingency plan. Likely this will
be an emergency release with fixes or the recommendation to consider the
upgrade aborted and revert back to the previous release of AtomOne (v3.3.0).

## Governance votes

The following items summarize the voting options and what it means for this
proposal:

YES - You agree that the AtomOne chain should be updated with this release.
NO - You disagree that the AtomOne chain should be updated with this release.
ABSTAIN - You wish to contribute to the quorum but you formally decline to vote
either for or against the proposal.
