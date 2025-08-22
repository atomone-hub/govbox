The AtomOne v3 release is a major release that will initially target the
testnet. The on-chain voting period typically lasts 3 days.

On governance vote approval, validators will be required to update the AtomOne
chain binary at the halt-height specified in the on-chain proposal

# Release Binary & Upgrade Resources

IMPORTANT: Note that AtomOne v3.0.1 binary MUST be used.

The release can be found [here](https://github.com/atomone-hub/atomone/releases/tag/v3.0.1).

The upgrade guide can be found [here](https://github.com/atomone-hub/atomone/blob/main/UPGRADING.md).

# Proposed Release Contents

This release introduces the following major new features:

- Add upgrade code to mint photon from 50% of bond denom funds of Community Pool and 90% of Treasury DAO address #157
- Make `x/gov` quorum dynamic
- Add the `x/dynamicfee` module and use the EIP-15559 AIMD algorithm
- Make `x/gov` proposals deposits dynamic
- Burn `x/gov` proposals deposit if percentage of no votes > params.BurnDepositNoThreshold when tallying

The other changes can be found in the changelog [here](https://github.com/atomone-hub/atomone/blob/main/CHANGELOG.md#v301).

# Testing

The v3 release has gone through rigorous testing, including e2e tests and
integration tests.

# Potential risk factors

Although very extensive testing and simulation will have taken place there
always exists a risk that the AtomOne experience problems due to potential bugs
or errors from the new features. In the case of serious problems, validators
should stop operating the network immediately. Coordination with validators
will happen in the #testnet-private channel of the AtomOne Community Discord to
create and execute a contingency plan. Likely this will be an emergency release
with fixes or the recommendation to consider the upgrade aborted and revert
back to the previous release of AtomOne (v2.0.0)

# Governance votes

The following items summarize the voting options and what it means for this
proposal:

YES - You agree that the AtomOne chain should be updated with this release.

NO - You disagree that the AtomOne chain should be updated with this release.

ABSTAIN - You wish to contribute to the quorum but you formally decline to vote
either for or against the proposal.
