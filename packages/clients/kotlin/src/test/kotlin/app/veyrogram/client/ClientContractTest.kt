package app.veyrogram.client

import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put
import kotlinx.serialization.json.Json
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** Contract tests for the generated Kotlin client (Story 1.11). A fake [Transport] stands in for the network. */
class ClientContractTest {
    private class Fake(val status: Int, val body: String, val headers: Map<String, String> = emptyMap()) : Transport {
        var last: HttpRequest? = null
        override fun execute(request: HttpRequest): HttpResponse { last = request; return HttpResponse(status, headers, body) }
    }

    private fun client(t: Transport, token: String? = "tok") = VeyroGramClient("https://api.example.test/", t) { token }

    @Test fun loginSendsTheReservedWordFieldAndNoBearerToken() {
        val t = Fake(200, """{"access_token":"a","token_type":"Bearer","expires_in":900,"refresh_token":"r","user_id":"u1","device_id":"d1","is_new_user":true,"future_field":1}""")
        val res = client(t, token = null).login(LoginRequest("+8613800000000", "123456", LoginRequestDevice("dev-1", "android")))
        assertEquals("a", res.accessToken)
        assertTrue(res.isNewUser)
        val req = t.last!!
        assertEquals("POST", req.method)
        assertEquals("https://api.example.test/v1/auth/login", req.url)
        assertNull(req.headers["Authorization"], "login must not carry a bearer token")
        val sent = Json.parseToJsonElement(req.body!!).jsonObject
        assertEquals("android", sent["device"]!!.jsonObject["class"]!!.jsonPrimitive.content, "the `class` field must be serialised under its contract name")
    }

    @Test fun authenticatedCallsCarryTheBearerToken() {
        val t = Fake(200, """{"items":[]}""")
        client(t).listDevices()
        assertEquals("Bearer tok", t.last!!.headers["Authorization"])
    }

    @Test fun signedOutCallNeverReachesTheNetwork() {
        val t = Fake(200, "{}")
        val e = assertFailsWith<ApiException> { client(t, token = null).listDevices() }
        assertEquals("not_signed_in", e.code)
        assertNull(t.last, "no request may be sent without a token")
    }

    @Test fun errorEnvelopeBecomesOneTypedError() {
        val t = Fake(403, """{"code":"forbidden","message":"nope","trace_id":"tr-1"}""")
        val e = assertFailsWith<ApiException> { client(t).listDevices() }
        assertEquals(403, e.status); assertEquals("forbidden", e.code); assertEquals("tr-1", e.traceId)
    }

    @Test fun nonJsonErrorBodyStillGivesATypedError() {
        val t = Fake(502, "<html>bad gateway</html>", mapOf("X-Request-Id" to "req-9"))
        val e = assertFailsWith<ApiException> { client(t).listDevices() }
        assertEquals(502, e.status); assertEquals("http_502", e.code); assertEquals("req-9", e.traceId)
    }

    @Test fun unexpectedSuccessStatusIsAnError() {
        val e = assertFailsWith<ApiException> { client(Fake(200, "{}")).logout() } // logout expects 204
        assertEquals(200, e.status)
    }

    @Test fun openMessagePayloadKeepsItsExtraFields() {
        val t = Fake(200, """{"message_id":"m1","message_seq":7,"client_msg_no":"c1"}""")
        val payload = buildJsonObject { put("type", 1); put("content", "hello") }
        client(t).sendMessage(SendRequest("u2", "c1", payload))
        val sent = Json.parseToJsonElement(t.last!!.body!!).jsonObject["payload"]!!.jsonObject
        assertEquals("hello", sent["content"]!!.jsonPrimitive.content, "content must not be dropped from the open payload")
        val m = Json.decodeFromString<Message>("""{"message_id":"m1","message_seq":1,"client_msg_no":"c","from_uid":"u","timestamp":1,"payload":{"type":1,"content":"x","extra":true}}""")
        assertEquals("x", m.payload!!.get("content")!!.jsonPrimitive.content)
    }

    @Test fun queryValuesAreEncodedAndNullsOmitted() {
        val t = Fake(200, """{"peer":"a b","messages":[],"next_seq":0,"has_more":false}""")
        client(t).syncMessages(peer = "a b&c", afterSeq = 5)
        assertEquals("https://api.example.test/v1/sync/messages?peer=a%20b%26c&after_seq=5", t.last!!.url)
    }

    @Test fun pathVariablesAreEncoded() {
        val t = Fake(204, "")
        client(t).blockUser("u/1 x")
        assertEquals("https://api.example.test/v1/blocks/u%2F1%20x", t.last!!.url)
        assertEquals("PUT", t.last!!.method)
    }
}
