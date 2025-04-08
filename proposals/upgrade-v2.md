The AtomOne v2 release is a major release that will follow the standard
governance process by initially submitting this post on the CommonWealth
AtomOne forum. After collecting forum feedback (~1 week) and adapting the
proposal as required, a governance proposal will be sent to the AtomOne chain
for voting. The on-chain voting period typically lasts 3 weeks.

On governance vote approval, validators will be required to update the AtomOne
chain binary at the halt-height specified in the on-chain proposal

## Release Binary & Upgrade Resources

IMPORTANT: Note that AtomOne v2 binary MUST be used.

The release can be found [here](https://github.com/atomone-hub/atomone/releases/tag/v2.0.0-rc1).

The upgrade guide can be found [here](https://github.com/atomone-hub/atomone/blob/main/UPGRADING.md).

## Proposed Release Contents

This release introduces the following major new feature:

- Add the `x/photon` module

Although this feature will make the `photon` token the sole fee token, the v2
upgrade will implement this change gradually. Rather than immediately rejecting
transactions using `atone` tokens for fees, we'll provide a transition period
for users to switch to `photon`. During this time, both `photon` and `atone`
    will be accepted as fee tokens. A subsequent parameter change proposal will
    then establish `photon` as the exclusive fee token.

The other changes can be found in the changelog [here](https://github.com/atomone-hub/atomone/blob/main/CHANGELOG.md#v200).

## Testing and Testnets

The v2 release has gone through rigorous testing, including e2e tests and
integration tests. 

Validators and node operators have joined a public testnet to participate in a
test upgrade to a release candidate before the AtomOne upgrades to the final
release.

## Potential risk factors

Although very extensive testing and simulation will have taken place there
always exists a risk that the AtomOne experience problems due to potential bugs
or errors from the new features. In the case of serious problems, validators
should stop operating the network immediately. Coordination with validators
will happen in the #validator-private channel of the AtomOne Community Discord
to create and execute a contingency plan. Likely this will be an emergency
release with fixes or the recommendation to consider the upgrade aborted and
revert back to the previous release of AtomOne (v1.1.2)

## Governance votes

The following items summarize the voting options and what it means for this
proposal:

YES - You agree that the AtomOne chain should be updated with this release.

NO - You disagree that the AtomOne chain should be updated with this release.

ABSTAIN - You wish to contribute to the quorum but you formally decline to vote
either for or against the proposal.
