# Proposal

This proposal aims to update the `x/photon` module parameters in order to make
PHOTON the only fee token, except for the `MsgMintPhoton` message.

# Motivation

While PHOTON was designed to be the only fee token, we decided to temporarily
continue accepting ATONE for fees to ensure a smooth transition to AtomOne v2.

Given the 3 weeks of voting period for a proposal, we think it is time to
schedule the end of this transition time and update the `x/photon` module
parameters accordingly.

# Implementation

The current x/photon module parameters are:

```json
{
  "params": {
    "mint_disabled": false,
    "tx_fee_exceptions": [
      "*"
    ]
  }
}
```

The proposal will change them into:

```json
{
  "params": {
    "mint_disabled": false,
    "tx_fee_exceptions": [
      "/atomone.photon.v1.MsgMintPhoton"
    ]
  }
}
```

No other changes are required in validator configuration, if they have followed
the [v2 upgrade guide](https://github.com/atomone-hub/atomone/blob/7c88091ef64628baa098b2251000f9af0f81c049/UPGRADING.md),
their `minimum-gas-prices` should already contain a value expressed in ATONE
and a value expressed in PHOTON.

# Consequences

Once this proposal passes, a transaction that contains other messages than
`MsgMintPhoton` will be rejected if the fee token is not PHOTON. The expected
error is `invalid fee token: fee denom uatone is not allowed`.

# Voting options

Yes: You are in favor of having the PHOTON as the only fee token, as stipulated
in the AtomOne Constitution.
No: You are against having PHOTON as the only fee token.
ABSTAIN: You wish to contribute to the quorum but you formally decline to vote
either for or against the proposal.
