package app.veyrogram.improbe;

import android.content.Context;
import android.os.Bundle;
import androidx.test.platform.app.InstrumentationRegistry;
import com.xinbida.wukongim.WKIM;
import com.xinbida.wukongim.entity.WKChannelType;
import com.xinbida.wukongim.entity.WKMsg;
import com.xinbida.wukongim.message.type.WKConnectStatus;
import com.xinbida.wukongim.message.type.WKSendMsgResult;
import com.xinbida.wukongim.msgmodel.WKTextContent;
import org.json.JSONObject;
import org.junit.Test;

import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertTrue;

/**
 * One IM client (WuKongIMAndroidSDK, the exact version the Android app ships). Driven by instrumentation arguments:
 * addr=host:port uid token mode=connect|bad|recv|send [peer] [text]. Each result is reported with sendStatus (event=...) so the Go
 * contract test (services/api/internal/platform/testim) can read it from `am instrument -r`.
 */
public class ProbeTest {
    private static void emit(String event, String k, String v) {
        Bundle b = new Bundle();
        b.putString("event", event);
        if (k != null) b.putString(k, v);
        InstrumentationRegistry.getInstrumentation().sendStatus(0, b);
    }

    @Test
    public void run() throws Exception {
        Bundle a = InstrumentationRegistry.getArguments();
        String addr = a.getString("addr"), uid = a.getString("uid"), token = a.getString("token"), mode = a.getString("mode");
        String peer = a.getString("peer"), text = a.getString("text");
        String host = addr.substring(0, addr.lastIndexOf(':'));
        int port = Integer.parseInt(addr.substring(addr.lastIndexOf(':') + 1));
        Context ctx = InstrumentationRegistry.getInstrumentation().getTargetContext();

        WKIM im = WKIM.getInstance();
        // The SDK builds a Handler on the creating thread's Looper, exactly as the app does on its main thread.
        InstrumentationRegistry.getInstrumentation().runOnMainSync(() -> {
            im.setFileCacheDir("improbe");
            im.setDebug("1".equals(a.getString("debug")));
            // The app answers the SDK's conversation sync over HTTP; this contract test has no business API, so it answers "nothing to sync".
            // Without an answer the SDK stays in SyncMsg and does not send.
            im.getConversationManager().addOnSyncConversationListener((lastMsgSeqs, msgCount, version, back) -> {
                com.xinbida.wukongim.entity.WKSyncChat chat = new com.xinbida.wukongim.entity.WKSyncChat();
                chat.uid = uid; chat.cmds = new java.util.ArrayList<>(); chat.conversations = new java.util.ArrayList<>(); chat.channel_status = new java.util.ArrayList<>();
                back.onBack(chat);
            });
            im.init(ctx, uid, token);
            im.getConnectionManager().addOnGetIpAndPortListener(l -> l.onGetSocketIpAndPort(host, port));
        });

        CountDownLatch connected = new CountDownLatch(1), rejected = new CountDownLatch(1), done = new CountDownLatch(1);
        AtomicReference<String> reason = new AtomicReference<>();
        im.getConnectionManager().addOnConnectionStatusListener("probe", (status, why) -> {
            emit("status", "status", status + ":" + why);
            if (status == WKConnectStatus.success) connected.countDown();
            if (status == WKConnectStatus.fail || status == WKConnectStatus.kicked) { reason.set(why); rejected.countDown(); }
        });
        AtomicReference<WKMsg> got = new AtomicReference<>();
        im.getMsgManager().addOnNewMsgListener("probe", msgs -> {
            if (msgs != null && !msgs.isEmpty()) { got.set(msgs.get(0)); done.countDown(); }
        });
        AtomicReference<WKMsg> ack = new AtomicReference<>();
        im.getMsgManager().addOnSendMsgAckListener("probe", m -> { ack.set(m); done.countDown(); });

        InstrumentationRegistry.getInstrumentation().runOnMainSync(() -> im.getConnectionManager().connection());
        if (mode.equals("bad")) {
            // Observed with WuKongIMAndroidSDK 1.5.0: the server drops the connection and the SDK retries with back-off; it does not always
            // surface an explicit auth-failure status. The contract is therefore "never reaches Connected", not "reports ReasonAuthFail".
            rejected.await(15, TimeUnit.SECONDS);
            Thread.sleep(3000);
            assertEquals("a wrong token must never connect", 1, connected.getCount());
            emit("not_connected", "reason", String.valueOf(reason.get()));
            InstrumentationRegistry.getInstrumentation().runOnMainSync(() -> im.getConnectionManager().disconnect(true));
            return;
        }
        assertTrue("the SDK must connect with the issued token", connected.await(20, TimeUnit.SECONDS));
        emit("connected", null, null);
        if (mode.equals("send")) {
            InstrumentationRegistry.getInstrumentation().runOnMainSync(() -> im.getMsgManager().sendMessage(new WKTextContent(text), peer, WKChannelType.PERSONAL));
            assertTrue("no send ack", done.await(20, TimeUnit.SECONDS));
            assertEquals("send must be acknowledged as success", WKSendMsgResult.send_success, ack.get().status);
            emit("ack", "status", String.valueOf(ack.get().status));
        } else if (mode.equals("recv")) {
            assertTrue("no message received", done.await(60, TimeUnit.SECONDS));
            WKMsg m = got.get();
            String t = new JSONObject(m.content).optString("content");
            Bundle b = new Bundle();
            b.putString("event", "message"); b.putString("from", m.fromUID); b.putString("text", t);
            InstrumentationRegistry.getInstrumentation().sendStatus(0, b);
        }
        InstrumentationRegistry.getInstrumentation().runOnMainSync(() -> im.getConnectionManager().disconnect(true));
    }
}
