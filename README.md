# CosmWasm IBC Example

This is a simple IBC enabled CosmWasm smart contract. It expects to be
deployed on two chains and, when prompted, will send messages to its
counterpart. It then counts the number of times messages have been
received on both sides.

At a high level, to use this contract:

1. Store and instantiate the contract on two IBC enabled chains. We
   will call these chains chain A and chain B.
2. Configure and run a relayer to connect the two contracts.
3. Execute the `Increment {}` method on one contract to increment the
   send a message and increment the count on the other one.
4. Use the `GetCount { connection }` query to determine the message
   count for a given connection.

This repo also contains a demo contract which makes an infinite loop
of contract calls over IBC. See the README on the
[`zeke/ibc-replay`](https://github.com/ezekiiel/cw-ibc-example/tree/zeke/ibc-replay)
branch for more information and an integration test that demonstrates
this.

## Background

To connect two CosmWasm contracts over IBC you must establish an IBC
channel between them. The IBC channel establishment process uses a
four way handshake. Here is a summary of the steps:

1. `OpenInit` Hello chain B, here is information that you can use to
   verify I am chain A. Do you have information I can use?
2. `OpenTry` Hello chain A, I have verified that you are who you say
   you are. Here is my verification information.
3. `OpenAck` Hello chain B. Thank you for that information I have
   verified you are who you say you are. I am now ready to talk.
4. `OpenConfirm` Hello chain A. I am also now ready to talk.

Once the handshake has been completed a channel will be established
that the ibc messages may be sent over. In order to do a handshake and
receive IBC messages your contract must implement the following entry
points (see `src/ibc.rs`):

1. `ibc_channel_open` - Handles the `OpenInit` and `OpenTry` handshake
   steps.
2. `ibc_channel_connect` - Handles the `OpenAck` and `OpenConfirm`
   handshake steps.
3. `ibc_channel_close` - Handles the closing of an IBC channel by the
   counterparty.
4. `ibc_packet_receive` - Handles receiving IBC packets from the
   counterparty.
5. `ibc_packet_ack` - Handles ACK messages from the countarparty. This
   is effectively identical to the ACK message type in
   [TCP](https://developer.mozilla.org/en-US/docs/Glossary/TCP_handshake).
6. `ibc_packet_timeout` - Handles packet timeouts.

Having implemented these methods, once you instantiate an instance of
the contract it will be assigned a port. Ports identify a receiver on
a blockchain in much the same way as ports identify applications on a
computer.

You can find the port that has been assigned to your contract by
running `junod query wasm contract <ADDRESS>` and inspecting the
`ibc_port_id` field. For example:

```
$ junod query wasm contract juno1r8k4hf7umksu9w53u4sz0jsla5478am6yxr0mhkuvp00yvtmxexsj8wazt
address: juno1r8k4hf7umksu9w53u4sz0jsla5478am6yxr0mhkuvp00yvtmxexsj8wazt
contract_info:
  admin: ""
  code_id: "1377"
  created: null
  creator: juno1m7a7nva00p82xr0tssye052r8sxsxvcy2v5qz6
  extension: null
  ibc_port_id: wasm.juno1r8k4hf7umksu9w53u4sz0jsla5478am6yxr0mhkuvp00yvtmxexsj8wazt
  label: ekez-cw-ibc-example
```

To establish a connecton between two contracts you will need to set
up a relayer. If you chose to use
[hermes](https://hermes.informal.systems), after configuration the
command to establish a connection is:

```
hermes create channel --a-chain uni-3 --b-chain juno-1 --a-port wasm.juno1r8k4hf7umksu9w53u4sz0jsla5478am6yxr0mhkuvp00yvtmxexsj8wazt --b-port wasm.juno1fsay0zux2vkyrsqpepd08q2vlytrfu7gsqnapsfl9ge8mp6fvx3qf062q9 --channel-version counter-1
```

Then, to start the relayer:

```
hermes start
```

Note that you will need to [configure
hermes](https://hermes.informal.systems/config.html) for the chains
you are relaying between before these commands may be run.

Once the relayer is running, make note of the channel ID that has been
established. You will use this when telling the contract to send
packets. This is needed because one contract may be connected to
multiple channels. You can not assume that your channel is the only
one connected at a given time.

For example, to increment the count on the counterparty chain over a connection between local
channel-72 and remote channel-90:

```
junod tx wasm execute juno1r8k4hf7umksu9w53u4sz0jsla5478am6yxr0mhkuvp00yvtmxexsj8wazt '{"increment": { "channel": "channel-72" }}' --from ekez --gas auto --gas-adjustment 2 --fees 12000ujunox
```

Then, switching to the other chain, we can query that count by
running:

```
junod query wasm contract-state smart juno1fsay0zux2vkyrsqpepd08q2vlytrfu7gsqnapsfl9ge8mp6fvx3qf062q9 '{"get_count": {"channel": "channel-90"}}'
```

## Troubleshooting

1. Packets may take over a minute to be relayed. If your packet is not
   sent instantly it is probably OK. Feel free to take a break and
   come back.
2. Hermes will not pick up packets that were sent while it was not
   running. Start your relayer and wait for the log line `Hermes has
   started` before sending packets.
