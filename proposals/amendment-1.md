# AtomOne Constitution Amendment

The AtomOne Constitution is a living document, and as such need to be meticulously maintained and updated whenever required, while focusing on respecting its founding principles and guidelines.
With this proposal we introduce several changes to the AtomOne Constitution, and therefore request the AtomOne governance vote to amend it.

# What is being changed

Several changes to the text are just intended to fix typos, make punctuation uniform, remove leftover characters - such as spaces - or add references to other parts of the text whenever appropriate. Without listing the full set of changes, we hereby list the most relevant, i.e. all the changes except the aforementioned kinds.
The complete diff can be found here: [https://github.com/atomone-hub/genesis/pull/198/files](https://github.com/atomone-hub/genesis/pull/198/files)

## Article 2, Section 3

The original text:

> To prevent spam and to ensure the quality of proposals, proposals must have a sufficient amount of burn deposit before voting can begin. The minimum amount of burn deposit needed shall self-adjust to target on average 1 proposal per 2week period.

Is changed to:

> To prevent spam and to ensure the quality of proposals, proposals must have a sufficient amount of burn deposit before voting can begin. The minimum amount of burn deposit needed shall self-adjust based on the number of active proposals on the chain. The target number of simultaneously active proposals is initially set to 2, hence the amount of deposit required to submit a proposal on chain will increase if there are more than 2 active proposals.

The change is intended to align the Constitution with the proposed implementation of the Dynamic Deposit voted in [Proposal 9](https://gov.atom.one/proposals/9).

## Article 2, Section 3

The original text:

> The quorum necessary for a proposal to be valid is 40%. The denominator shall be the number of bonded ATONE tokens.

Is changed to:

> The quorum necessary for a proposal to be valid shall self-adjust between a minimum and a maximum value, based on historical participation. The parameters dictating the ways in which the quorum adjusts - including minimum and maximum value - can be modified through governance. The denominator to compute the quorum shall be the number of bonded ATONE tokens.

This change is intended to fix an inconsistency between Constitution and implementation, and align the text with the Dynamic Quorum voted in [Proposal 11](https://gov.atom.one/proposals/11).

## Article 2, Section 4.b

The original text:

> All Core DAOs and their sub-DAOs shall be composed of Cosmonauts, and the DAO Councils be composed of Citizens. All Cosmonauts and Citizens of these DAOs must have public and known real human identities.

Is changed to:

> All Core DAOs, their sub-DAOs and the DAO Councils shall be composed of Citizens. All Citizens of these DAOs must have public and known real human identities.

This change is intended to restrict DAO participation to Citizens instead of the more broad Cosmonauts.

## Article 2, Section 5

The original text:

> Every Citizen allows any Cosmonaut to modify their pro-rata airdrop portion by partial or full slashing (or by proportionate rewards) based on their cryptographic voting activity according to well defined principles at any time.

Is changed to:

> Every ATONE holder allows any Cosmonaut to modify their pro-rata airdrop portion by partial or full slashing (or by proportionate rewards) based on their cryptographic voting activity according to well defined principles at any time.

This change is intended to broaden the provision to encompass any ATONE holder, and not just Citizens.

## Article 3, Section 2

The original text

> Inflated ATONE tokens are paid to bonded ATONE holders in proportion to each delegator's staking amount.

Is changed to:

> Inflated ATONE tokens are paid to bonded ATONE holders.

This slight modification is intended to accomodate the Nakamoto Bonus voted in [Proposal 12](https://gov.atom.one/proposals/12).

## Article 3, Section 2

The original text

> redelegation shall be allowed twice per ATONE Unbonding Period

Is changed to:

> redelegation shall be allowed up to twice per ATONE Unbonding Period

This modification is intended to align the Constitution with the current implementation while still retaining the restriction, but only set as an upper bound.

# Constitution Amendment voting procedure

It is here noted that for a Constitution Amendment to pass the vote need to reach a Constitutional Majority, which is set at 90%. Therefore the Amendment will only go into effect if the threshold of 90% YES votes is reached.

# Governance votes

The following items summarize the voting options and what it means for this proposal:

YES - You agree that the AtomOne Constitution should be amended with the proposed changes.
NO - You disagree that the AtomOne Constitution should be amended with the proposed changes.
ABSTAIN - You wish to contribute to the quorum but you formally decline to vote either for or against the proposal.
