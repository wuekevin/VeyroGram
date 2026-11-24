// One IM client process (the SDK keeps a process-wide singleton, so each user runs in its own process).
// Usage: node client.mjs <ws-url> <uid> <token> <recv|send|connect> [peer] [text] [clientMsgNo]
// Prints one JSON object per line: {event: "status"|"connected"|"message"|"sent"|"error", ...}. Exit 0 on success.
import WKSDKModule, { Channel, ChannelTypePerson, ConnectStatus } from "wukongimjssdk";
const WKSDK = WKSDKModule.default ?? WKSDKModule;
const [, , addr, uid, token, mode, peer, text, clientMsgNo] = process.argv;
const out = (o) => console.log(JSON.stringify(o));
const sdk = WKSDK.shared();
sdk.config.addr = addr;
sdk.config.uid = uid;
sdk.config.token = token;
const timer = setTimeout(() => { out({ event: "error", error: "timeout" }); process.exit(2); }, 20000);

sdk.connectManager.addConnectStatusListener((status, reasonCode) => {
  out({ event: "status", status: ConnectStatus[status] ?? status, reasonCode: reasonCode ?? null });
  if (status === ConnectStatus.Connected) {
    out({ event: "connected" });
    if (mode === "connect") { clearTimeout(timer); sdk.connectManager.disconnect(); process.exit(0); }
    if (mode === "send") {
      const c = sdk.newMessageText(text);
      sdk.chatManager.send(c, new Channel(peer, ChannelTypePerson)).then((m) => {
        out({ event: "sent", clientMsgNo: m.clientMsgNo, status: m.status });
      }).catch((e) => { out({ event: "error", error: String(e) }); process.exit(3); });
    }
  } else if (status === ConnectStatus.ConnectFail || status === ConnectStatus.ConnectKick) {
    if (mode === "connect") { clearTimeout(timer); out({ event: "rejected", reasonCode: reasonCode ?? null }); process.exit(4); }
  }
});
sdk.chatManager.addMessageStatusListener((ack) => {
  out({ event: "ack", clientSeq: ack.clientSeq, messageID: String(ack.messageID), messageSeq: ack.messageSeq, reasonCode: ack.reasonCode });
  if (mode === "send") { clearTimeout(timer); process.exit(ack.reasonCode === 1 ? 0 : 5); }
});
sdk.chatManager.addMessageListener((m) => {
  out({ event: "message", from: m.fromUID, channelID: m.channel.channelID, text: m.content?.text ?? null, messageSeq: m.messageSeq, clientMsgNo: m.clientMsgNo });
  if (mode === "recv") { clearTimeout(timer); process.exit(0); }
});
sdk.connectManager.connect();
