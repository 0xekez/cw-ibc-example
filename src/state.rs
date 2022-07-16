use cw_storage_plus::Map;

// Mapping between connections and the counter on that connection.
pub const CONNECTION_COUNTS: Map<String, u32> = Map::new("connection_counts");
