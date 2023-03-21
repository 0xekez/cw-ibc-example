use cosmwasm_schema::{cw_serde, QueryResponses};

#[cw_serde]
pub struct InstantiateMsg {}

#[cw_serde]
pub enum ExecuteMsg {
    Increment { channel: String },
}

#[cw_serde]
pub enum IbcExecuteMsg {
    Increment {},
}

#[cw_serde]
#[derive(QueryResponses)]
pub enum QueryMsg {
    // GetCount returns the current count as a json-encoded number
    #[returns(crate::msg::GetCountResponse)]
    GetCount {
        // The ID of the LOCAL channel you'd like to query the count
        // for.
        channel: String,
    },
    // GetTimeoutCount returns the number of timeouts have occured on
    // the LOCAL channel `channel`.
    #[returns(crate::msg::GetCountResponse)]
    GetTimeoutCount { channel: String },
}

// We define a custom struct for each query response
#[cw_serde]
pub struct GetCountResponse {
    pub count: u32,
}
