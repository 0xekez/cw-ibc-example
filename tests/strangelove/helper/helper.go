package helper

type QueryResponse struct {
	Data GetCountQuery `json:"data"`
}

type GetCountQuery struct {
	Count uint32 `json:"count"`
}

type QueryMsg struct {
	GetCount        *GetCount `json:"get_count,omitempty"`
	GetTimeoutCount *GetCount `json:"get_timeout_count,omitempty"`
}

type GetCount struct {
	Channel string `json:"channel"`
}

type KvPair struct {
	Key   string // hex encoded string
	Value string // b64 encoded json
}

type ContractStateResp struct {
	Models []KvPair
}

func Ptr[T any](v T) *T {
	return &v
}
