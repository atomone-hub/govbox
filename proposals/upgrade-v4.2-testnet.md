# AtomOne v4.2 testnet upgrade

The AtomOne v4.2 release is a minor release that targets the testnet first.
The on-chain voting period typically lasts 3 days.

On governance vote approval, validators will be required to update the AtomOne
chain binary at the halt-height specified in the on-chain proposal.

## Release Binary & Upgrade Resources

IMPORTANT: Note that AtomOne v4.2.0-rc.2 binary MUST be used.

The release can be found [here](https://github.com/atomone-hub/atomone/releases/tag/v4.2.0-rc.2).
The changelog can be found [here](https://github.com/atomone-hub/atomone/compare/v4.1.0...v4.2.0-rc.2).

## Proposed Release Contents

This release includes the following changes since v4.1.0:

- Bump the AtomOne SDK to v0.500.6
- `x/coredaos`: migrate away from the AtomOne `x/gov` wrapper, move the
  oversight-DAO bundling check to a gov hook, reject proposals that delegate
  `x/coredaos` `MsgUpdateParams` via authz, and defer `MsgVetoProposal`
  deposit/vote cleanup to the EndBlocker
- `x/photon`: prevent bundling the wildcard symbol in `TxFeeExceptions` with
  other exceptions
- `x/dynamicfee`: consume failed-tx gas in dynamic fee pricing
- `x/gov`: always validate quorum check params, and register the legacy IBC
  gov proposal content types
- `10-gno` light client: bind validator addresses to pubkeys and bound
  `PartSetHeader.Total` during conversion
- Add the v4.2 upgrade handler

## Testing

The v4.2 release candidate has gone through unit and e2e testing. This
proposal exercises the upgrade on the testnet before it is proposed on mainnet.

## Potential risk factors

Although extensive testing has taken place, there always exists a risk that
AtomOne experiences problems due to potential bugs. In the case of serious
problems, validators should stop operating the network immediately.
Coordination with validators will happen in the #testnet-private channel of the
AtomOne Community Discord to create and execute a contingency plan.

## Governance votes

The following items summarize the voting options and what it means for this
proposal:

YES - You agree that the AtomOne chain should be updated with this release.

NO - You disagree that the AtomOne chain should be updated with this release.

ABSTAIN - You wish to contribute to the quorum but you formally decline to vote
either for or against the proposal.
