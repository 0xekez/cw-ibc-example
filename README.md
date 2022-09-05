# CosmWasm IBC Infinite Loop

This is a CosmWasm IBC contract that loops infinitely periodically
waking itself up every couple blocks. So long as relayers continue to
relay its packets, it will forever call into itself and wake up. You
might use a variation of this contract to make cron jobs, or to be a
menace.

Here is how it works:

1. The contract is deployed on chains A and B.
2. A channel is established to relay packets between the two instances
   of the contract.
3. Side A sends a message to side B.
4. Upon receiving that message, side B sends an ACK back to side A.
5. In it's ACK handler, side A replays the message it previously sent
   to side B.
6. `goto 1.`

The code to replay the message being ACKed is quite simple as the ACK
contains the initial message that is being ACKed:

```rust
#[cfg_attr(not(feature = "library"), entry_point)]
pub fn ibc_packet_ack(
    _deps: DepsMut,
    env: Env,
    ack: IbcPacketAckMsg,
) -> Result<IbcBasicResponse, ContractError> {
    // Play it back. ;)
    Ok(IbcBasicResponse::new()
        .add_attribute("method", "ibc_packet_ack")
        .add_message(IbcMsg::SendPacket {
            channel_id: ack.original_packet.src.channel_id,
            data: ack.original_packet.data,
            timeout: IbcTimeout::with_timestamp(env.block.time.plus_seconds(300)),
        }))
}
```

If you don't believe me that this works, I've written a ts-relayer
integration test that will reproduce this. Notably, I have not checked
this in because I am lazy.

If you'd like to actually run this there are some docs on getting
ts-relayer set up
[here](https://github.com/confio/ts-relayer/blob/main/DEVELOPMENT.md). For
the test below, I've just replaced the `fund-relayer.spec.ts` file's
contents with these so that I didn't need to set up my own development
environment. You'll also need to run `osmosisd` and not `gaiad` from
those instructions to run this so that you have two CosmWasm chains
that can talk.

Anyway, here's the integration test:

```typescript
import test from 'ava';
import { Order } from 'cosmjs-types/ibc/core/channel/v1/channel';
import { readFileSync } from 'fs';
import { testutils } from '..';

import {
  CosmWasmSigner,
  fundAccount,
  osmosis,
  signingCosmWasmClient,
  wasmd,
} from '../helpers';
import { Link } from '../link';

const uploadContract = async (client: CosmWasmSigner, path: string) => {
  const wasm = readFileSync(path);
  const res = await client.sign.upload(
    client.senderAddress,
    wasm,
    'auto',
    'Upload contract'
  );
  return res.codeId;
};

test.serial('fund relayer', async (t) => {
  await fundAccount(osmosis, osmosis.faucet.address0, '50000000');
  await fundAccount(wasmd, wasmd.faucet.address0, '50000000');

  const path =
    '/Users/ekez/projects/cw-ibc-example/artifacts/cw_ibc_example.wasm';

  const wasmClient = await signingCosmWasmClient(wasmd, wasmd.faucet.mnemonic);
  const wasmCodeId = await uploadContract(wasmClient, path);
  console.debug(`wasm code id: ${wasmCodeId}`);

  const osmoClient = await signingCosmWasmClient(
    osmosis,
    osmosis.faucet.mnemonic
  );
  const osmoCodeId = await uploadContract(osmoClient, path);
  console.debug(`osmo code id: ${osmoCodeId}`);

  const osmoAddress = (
    await osmoClient.sign.instantiate(
      osmoClient.senderAddress,
      osmoCodeId,
      {},
      'osmo-example',
      'auto'
    )
  ).contractAddress;
  console.debug(`osmo address: ${osmoAddress}`);

  const wasmAddress = (
    await wasmClient.sign.instantiate(
      wasmClient.senderAddress,
      wasmCodeId,
      {},
      'wasm-example',
      'auto'
    )
  ).contractAddress;
  console.debug(`wasm address: ${wasmAddress}`);

  const { ibcPortId: wasmPort } = await wasmClient.sign.getContract(
    wasmAddress
  );
  const { ibcPortId: osmoPort } = await osmoClient.sign.getContract(
    osmoAddress
  );
  t.truthy(wasmPort);
  t.truthy(osmoPort);

  const [src, dst] = await testutils.setup(wasmd, osmosis);
  const link = await Link.createWithNewConnections(src, dst);
  const channelInfo = await link.createChannel(
    'A',
    wasmPort!,
    osmoPort!,
    Order.ORDER_UNORDERED,
    'counter-1'
  );

  console.debug(`channel info: ${JSON.stringify(channelInfo, undefined, 2)}`);

  const res = await wasmClient.sign.execute(
    wasmClient.senderAddress,
    wasmAddress,
    {
      increment: { channel: channelInfo.src.channelId },
    },
    'auto'
  );

  console.debug(
    `executed increment. res: ${JSON.stringify(res, undefined, 2)}`
  );

  // Repeat this call as many times as you'd like to see the loop.
  await link.relayAll();
  await link.relayAll();

  const osmoCount = await osmoClient.sign.queryContractSmart(osmoAddress, {
    get_count: { channel: channelInfo.dest.channelId },
  });

  console.log(`osmo side res is: ${JSON.stringify(osmoCount, undefined, 2)}`);

  // to make ava happy
  t.is(1, 1);
});
```
