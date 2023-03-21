#[cfg(not(feature = "library"))]
use cosmwasm_std::entry_point;
use cosmwasm_std::{
    to_binary, Binary, Deps, DepsMut, Env, IbcMsg, IbcTimeout, MessageInfo, Response, StdError,
    StdResult,
};
use cw2::set_contract_version;

use crate::error::ContractError;
use crate::msg::{ExecuteMsg, GetCountResponse, IbcExecuteMsg, InstantiateMsg, QueryMsg};
use crate::state::{CONNECTION_COUNTS, TIMEOUT_COUNTS};

const CONTRACT_NAME: &str = "crates.io:cw-ibc-example";
const CONTRACT_VERSION: &str = env!("CARGO_PKG_VERSION");

#[cfg_attr(not(feature = "library"), entry_point)]
pub fn instantiate(
    deps: DepsMut,
    _env: Env,
    _info: MessageInfo,
    _msg: InstantiateMsg,
) -> Result<Response, ContractError> {
    set_contract_version(deps.storage, CONTRACT_NAME, CONTRACT_VERSION)?;
    Ok(Response::new().add_attribute("method", "instantiate"))
}

#[cfg_attr(not(feature = "library"), entry_point)]
pub fn execute(
    _deps: DepsMut,
    env: Env,
    _info: MessageInfo,
    msg: ExecuteMsg,
) -> Result<Response, ContractError> {
    match msg {
        ExecuteMsg::Increment { channel } => Ok(Response::new()
            .add_attribute("method", "execute_increment")
            .add_attribute("channel", channel.clone())
            // outbound IBC message, where packet is then received on other chain
            .add_message(IbcMsg::SendPacket {
                channel_id: channel,
                data: to_binary(&IbcExecuteMsg::Increment {})?,
                // default timeout of two minutes.
                timeout: IbcTimeout::with_timestamp(env.block.time.plus_seconds(120)),
            })),
    }
}

/// called on IBC packet receive in other chain
pub fn try_increment(deps: DepsMut, channel: String) -> Result<u32, StdError> {
    CONNECTION_COUNTS.update(deps.storage, channel, |count| -> StdResult<_> {
        Ok(count.unwrap_or_default() + 1)
    })
}

#[cfg_attr(not(feature = "library"), entry_point)]
pub fn query(deps: Deps, _env: Env, msg: QueryMsg) -> StdResult<Binary> {
    match msg {
        QueryMsg::GetCount { channel } => to_binary(&query_count(deps, channel)?),
        QueryMsg::GetTimeoutCount { channel } => to_binary(&query_timeout_count(deps, channel)?),
    }
}

fn query_count(deps: Deps, channel: String) -> StdResult<GetCountResponse> {
    let count = CONNECTION_COUNTS
        .may_load(deps.storage, channel)?
        .unwrap_or_default();
    Ok(GetCountResponse { count })
}

fn query_timeout_count(deps: Deps, channel: String) -> StdResult<GetCountResponse> {
    let count = TIMEOUT_COUNTS
        .may_load(deps.storage, channel)?
        .unwrap_or_default();
    Ok(GetCountResponse { count })
}
