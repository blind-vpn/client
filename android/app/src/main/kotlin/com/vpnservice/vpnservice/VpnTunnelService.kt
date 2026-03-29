package com.vpnservice.vpnservice

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Intent
import android.net.VpnService
import com.wireguard.android.backend.GoBackend
import com.wireguard.android.backend.Tunnel
import com.wireguard.config.Config
import com.wireguard.config.InetEndpoint
import com.wireguard.config.InetNetwork
import com.wireguard.config.Interface
import com.wireguard.config.Peer
import com.wireguard.crypto.Key
import java.net.InetAddress

class VpnTunnelService : VpnService() {

    private var backend: GoBackend? = null
    private var tunnel: WgTunnel? = null
    var isRunning = false
        private set
    var currentEndpoint: String? = null
        private set

    companion object {
        const val NOTIFICATION_ID = 1
        const val CHANNEL_ID = "vpn_tunnel"
    }

    override fun onCreate() {
        super.onCreate()
        MainActivity.tunnelService = this
        createNotificationChannel()
        backend = GoBackend(this)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            "START" -> {
                val privateKey = intent.getStringExtra("privateKey") ?: return START_NOT_STICKY
                val serverPubKey = intent.getStringExtra("serverPublicKey") ?: return START_NOT_STICKY
                val serverEndpoint = intent.getStringExtra("serverEndpoint") ?: return START_NOT_STICKY
                val serverPort = intent.getIntExtra("serverPort", 51820)
                val tunnelAddress = intent.getStringExtra("tunnelAddress") ?: return START_NOT_STICKY
                val dns = intent.getStringExtra("dns") ?: return START_NOT_STICKY

                startForeground(NOTIFICATION_ID, buildNotification("Connecting..."))
                startTunnel(privateKey, serverPubKey, serverEndpoint, serverPort, tunnelAddress, dns)
            }
            "STOP" -> stopTunnel()
        }
        return START_STICKY
    }

    private fun startTunnel(
        privateKey: String,
        serverPubKey: String,
        serverEndpoint: String,
        serverPort: Int,
        tunnelAddress: String,
        dns: String
    ) {
        try {
            val config = Config.Builder()
                .setInterface(
                    Interface.Builder()
                        .parsePrivateKey(privateKey)
                        .addAddress(InetNetwork.parse("$tunnelAddress/32"))
                        .addDnsServer(InetAddress.getByName(dns))
                        .build()
                )
                .addPeer(
                    Peer.Builder()
                        .parsePublicKey(serverPubKey)
                        .parseEndpoint("$serverEndpoint:$serverPort")
                        .addAllowedIp(InetNetwork.parse("0.0.0.0/0"))
                        .addAllowedIp(InetNetwork.parse("::/0"))
                        .parsePersistentKeepalive("25")
                        .build()
                )
                .build()

            tunnel = WgTunnel("wg-vpn")
            backend?.setState(tunnel!!, Tunnel.State.UP, config)

            isRunning = true
            currentEndpoint = "$serverEndpoint:$serverPort"
            updateNotification("Connected to $serverEndpoint")
        } catch (e: Exception) {
            updateNotification("Connection failed: ${e.message}")
            stopTunnel()
        }
    }

    fun stopTunnel() {
        try {
            tunnel?.let { backend?.setState(it, Tunnel.State.DOWN, null) }
        } catch (_: Exception) {}
        tunnel = null
        isRunning = false
        currentEndpoint = null
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    override fun onDestroy() {
        stopTunnel()
        MainActivity.tunnelService = null
        super.onDestroy()
    }

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            "VPN Tunnel",
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "VPN connection status"
            setShowBadge(false)
        }
        val manager = getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(channel)
    }

    private fun buildNotification(text: String): Notification {
        return Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("VPN Service")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_lock_lock)
            .setOngoing(true)
            .build()
    }

    private fun updateNotification(text: String) {
        val manager = getSystemService(NotificationManager::class.java)
        manager.notify(NOTIFICATION_ID, buildNotification(text))
    }
}

class WgTunnel(private val name: String) : Tunnel {
    override fun getName(): String = name
    override fun onStateChange(newState: Tunnel.State) {}
}
