package com.vpnservice.vpnservice

import android.app.Activity
import android.content.Intent
import android.net.VpnService
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {
    private val CHANNEL = "vpnservice/tunnel"
    private var pendingResult: MethodChannel.Result? = null
    private var pendingConfig: Map<String, Any>? = null

    companion object {
        const val VPN_PERMISSION_REQUEST = 1001
        var tunnelService: VpnTunnelService? = null
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)

        // Register crypto channel for keypair generation
        CryptoChannel(flutterEngine.dartExecutor.binaryMessenger)

        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, CHANNEL).setMethodCallHandler { call, result ->
            when (call.method) {
                "connect" -> {
                    val config = mapOf(
                        "privateKey" to (call.argument<String>("privateKey") ?: ""),
                        "serverPublicKey" to (call.argument<String>("serverPublicKey") ?: ""),
                        "serverEndpoint" to (call.argument<String>("serverEndpoint") ?: ""),
                        "serverPort" to (call.argument<Int>("serverPort") ?: 51820),
                        "tunnelAddress" to (call.argument<String>("tunnelAddress") ?: ""),
                        "dns" to (call.argument<String>("dns") ?: "")
                    )

                    // Check VPN permission
                    val intent = VpnService.prepare(this)
                    if (intent != null) {
                        pendingResult = result
                        pendingConfig = config
                        startActivityForResult(intent, VPN_PERMISSION_REQUEST)
                    } else {
                        startTunnel(config, result)
                    }
                }
                "disconnect" -> {
                    tunnelService?.stopTunnel()
                    result.success(null)
                }
                "getStatus" -> {
                    val service = tunnelService
                    if (service != null && service.isRunning) {
                        result.success(mapOf(
                            "connected" to true,
                            "serverEndpoint" to (service.currentEndpoint ?: "")
                        ))
                    } else {
                        result.success(null)
                    }
                }
                else -> result.notImplemented()
            }
        }
    }

    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == VPN_PERMISSION_REQUEST) {
            if (resultCode == Activity.RESULT_OK) {
                pendingConfig?.let { config ->
                    pendingResult?.let { result ->
                        startTunnel(config, result)
                    }
                }
            } else {
                pendingResult?.error("VPN_PERMISSION_DENIED", "User denied VPN permission", null)
            }
            pendingResult = null
            pendingConfig = null
        }
    }

    private fun startTunnel(config: Map<String, Any>, result: MethodChannel.Result) {
        val intent = Intent(this, VpnTunnelService::class.java).apply {
            action = "START"
            putExtra("privateKey", config["privateKey"] as String)
            putExtra("serverPublicKey", config["serverPublicKey"] as String)
            putExtra("serverEndpoint", config["serverEndpoint"] as String)
            putExtra("serverPort", config["serverPort"] as Int)
            putExtra("tunnelAddress", config["tunnelAddress"] as String)
            putExtra("dns", config["dns"] as String)
        }
        startForegroundService(intent)
        result.success(null)
    }
}
