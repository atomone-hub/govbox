# Proposal

This proposal initializes the `key_rotation_fee` parameter of the `x/staking`
module to 100 ATONE, so that the consensus pubkey rotation feature becomes
usable on the testnet.

# Motivation

The testnet performed the v4 upgrade with a `v4.0.0-rc` binary, whose Cosmos SDK
version predates the consensus pubkey rotation feature. As a result its v4
upgrade handler did not initialize the `key_rotation_fee` parameter introduced by
that feature, while the final v4.0.0 binary run by mainnet does set it in
`MigrateStakingParams`.

The parameter is therefore unset on the testnet, and any `MsgRotateConsPubKey`
transaction fails because the rotation fee is an invalid (nil) coin. The current
on-chain parameters confirm the missing field:

```json
{
  "params": {
    "unbonding_time": "1814400s",
    "max_validators": 100,
    "max_entries": 7,
    "historical_entries": 10000,
    "bond_denom": "uatone",
    "min_commission_rate": "0.050000000000000000",
    "max_commission_rate": "0.050000000000000000"
  }
}
```

# Implementation

`MsgUpdateParams` requires all parameters to be supplied, so this proposal
repeats the current on-chain values unchanged and only adds:

```json
"key_rotation_fee": {
  "denom": "uatone",
  "amount": "100000000"
}
```

That is 100 ATONE, the same value `MigrateStakingParams` sets on mainnet, so
after this proposal the testnet staking parameters match the mainnet post-v4
state.

No coordinated upgrade, binary swap or node restart is required: the parameter is
the only piece of state the testnet is missing, and the rotation code is already
part of the binaries in use.

# Consequences

Once this proposal passes, validators can rotate their consensus pubkey with
`MsgRotateConsPubKey`, paying a 100 ATONE fee to the community pool. All other
staking parameters are left untouched, in particular the minimum and maximum
commission rates keep their current 5% value.

# Voting options

- Yes: You are in favor of initializing the `key_rotation_fee` parameter to 100
  ATONE, enabling consensus pubkey rotation on the testnet.
- No: You are against initializing the `key_rotation_fee` parameter.
- ABSTAIN: You wish to contribute to the quorum but you formally decline to vote
  either for or against the proposal.
