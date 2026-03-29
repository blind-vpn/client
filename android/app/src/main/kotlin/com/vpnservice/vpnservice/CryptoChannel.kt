package com.vpnservice.vpnservice

import com.wireguard.crypto.KeyPair
import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.MethodChannel

class CryptoChannel(messenger: BinaryMessenger) {
    private val channel = MethodChannel(messenger, "vpnservice/crypto")

    init {
        channel.setMethodCallHandler { call, result ->
            when (call.method) {
                "generateKeypair" -> {
                    try {
                        val keyPair = KeyPair()
                        result.success(mapOf(
                            "privateKey" to keyPair.privateKey.toBase64(),
                            "publicKey" to keyPair.publicKey.toBase64()
                        ))
                    } catch (e: Exception) {
                        result.error("KEYGEN_FAILED", e.message, null)
                    }
                }
                else -> result.notImplemented()
            }
        }
    }
}
