use cosmwasm_std::StdError;
use thiserror::Error;

#[derive(Error, Debug)]
pub enum Never {}

#[derive(Error, Debug)]
pub enum ContractError {
    #[error("{0}")]
    Std(#[from] StdError),

    #[error("Only unordered channels are supported.")]
    OrderedChannel {},

    #[error("Invalid IBC channel version. Got ({actual}), expected ({expected}).")]
    InvalidVersion { actual: String, expected: String },
}
