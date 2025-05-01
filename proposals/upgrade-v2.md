The AtomOne v2 release is a major release that will follow the standard
governance process. The on-chain voting period typically lasts 3 weeks. 

On governance vote approval, validators will be required to update the AtomOne
chain binary at the halt-height specified in the on-chain proposal

# Release Binary & Upgrade Resources

IMPORTANT: Note that AtomOne v2 binary MUST be used.

The release can be found [here](https://github.com/atomone-hub/atomone/releases/tag/v2.0.0).

The upgrade guide can be found [here](https://github.com/atomone-hub/atomone/blob/main/UPGRADING.md).

# Proposed Release Contents

This release introduces the following major new feature:

- Add the `x/photon` module

Although this feature will make the `photon` token the sole fee token, the v2
upgrade will implement this change gradually. Rather than immediately rejecting
transactions using `atone` tokens for fees, a transition period for users to
switch to `photon` will be provided. During this time, both `photon` and
`atone` will be accepted as fee tokens. Pending governance approval, a
subsequent parameter change proposal, will then establish `photon` as the
exclusive fee token.

The other changes can be found in the changelog [here](https://github.com/atomone-hub/atomone/blob/main/CHANGELOG.md#v200).

# Schedule

With a three-week voting period, in the event the proposal is supported, it
should pass around May 22nd. The upgrade will be expected to take place the
following week, at May 28th. The specific block halt-height is [3,318,000](https://www.mintscan.io/atomone/block/3318000).

# Testing and Testnets

The v2 release has gone through rigorous testing, including e2e tests and
integration tests. 

Validators and node operators have joined a [public
testnet](https://testnet.explorer.allinbits.services/atomone-testnet-1) to
participate in a test upgrade to a release candidate before the AtomOne
upgrades to the final release.

# Potential risk factors

Although very extensive testing and simulation will have taken place there
always exists a risk that the AtomOne experiences problems due to potential
bugs or errors from the new features. In the case of serious problems,
validators are recommended to stop operating the network immediately.
Coordination with validators will happen in the #validator-private channel of
the AtomOne Community Discord to create and execute a contingency plan. Likely
this will be an emergency release with fixes or the recommendation to consider
the upgrade aborted and revert back to the previous release of AtomOne (v1.1.2)

# Governance votes

The following items summarize the voting options and what it means for this
proposal:

YES - You agree that the AtomOne chain should be updated with this release.

NO - You disagree that the AtomOne chain should be updated with this release.

ABSTAIN - You wish to contribute to the quorum but you formally decline to vote
either for or against the proposal.
