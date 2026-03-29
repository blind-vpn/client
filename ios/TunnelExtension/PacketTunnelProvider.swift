import NetworkExtension
import os.log

/// Packet tunnel provider for the WireGuard VPN tunnel.
///
/// This extension runs in a separate process and manages the actual tunnel.
/// For production, integrate WireGuardKit which provides the WireGuard
/// protocol implementation via wireguard-go.
///
/// To add WireGuardKit:
/// 1. Add WireGuardKit Swift package: https://github.com/WireGuard/wireguard-apple
/// 2. Import WireGuardKit in this file
/// 3. Use WireGuardAdapter to start/stop the tunnel
class PacketTunnelProvider: NEPacketTunnelProvider {

    private let log = OSLog(subsystem: "com.vpnservice.vpnservice.tunnel", category: "tunnel")

    override func startTunnel(options: [String: NSObject]?, completionHandler: @escaping (Error?) -> Void) {
        os_log("Starting tunnel", log: log, type: .info)

        guard let config = (protocolConfiguration as? NETunnelProviderProtocol)?.providerConfiguration,
              let wgConfig = config["wgConfig"] as? String else {
            completionHandler(NSError(domain: "com.vpnservice", code: 1,
                                      userInfo: [NSLocalizedDescriptionKey: "Missing WireGuard config"]))
            return
        }

        // Parse the WireGuard config to extract tunnel parameters
        let lines = wgConfig.components(separatedBy: "\n")
        var address: String?
        var dns: String?

        for line in lines {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if trimmed.hasPrefix("Address") {
                address = trimmed.components(separatedBy: "=").last?.trimmingCharacters(in: .whitespaces)
            }
            if trimmed.hasPrefix("DNS") {
                dns = trimmed.components(separatedBy: "=").last?.trimmingCharacters(in: .whitespaces)
            }
        }

        // Configure the tunnel network settings
        let tunnelSettings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "10.66.0.1")

        if let address = address {
            let components = address.components(separatedBy: "/")
            let ip = components[0]
            let mask = components.count > 1 ? components[1] : "32"
            tunnelSettings.ipv4Settings = NEIPv4Settings(
                addresses: [ip],
                subnetMasks: [cidrToMask(Int(mask) ?? 32)]
            )
            tunnelSettings.ipv4Settings?.includedRoutes = [NEIPv4Route.default()]
        }

        if let dns = dns {
            tunnelSettings.dnsSettings = NEDNSSettings(servers: [dns])
        }

        tunnelSettings.mtu = 1280 as NSNumber

        setTunnelNetworkSettings(tunnelSettings) { error in
            if let error = error {
                os_log("Failed to set tunnel settings: %{public}@", log: self.log, type: .error, error.localizedDescription)
                completionHandler(error)
                return
            }

            // TODO: Initialize WireGuardKit adapter here:
            //
            // let adapter = WireGuardAdapter(with: self) { ... }
            // adapter.start(tunnelConfiguration: tunnelConfig) { error in
            //     completionHandler(error)
            // }
            //
            // For now, tunnel settings are configured but the WireGuard
            // protocol layer requires WireGuardKit.

            os_log("Tunnel started", log: self.log, type: .info)
            completionHandler(nil)
        }
    }

    override func stopTunnel(with reason: NEProviderStopReason, completionHandler: @escaping () -> Void) {
        os_log("Stopping tunnel, reason: %{public}d", log: log, type: .info, reason.rawValue)

        // TODO: adapter.stop { completionHandler() }

        completionHandler()
    }

    override func handleAppMessage(_ messageData: Data, completionHandler: ((Data?) -> Void)?) {
        completionHandler?(nil)
    }

    private func cidrToMask(_ cidr: Int) -> String {
        var mask: UInt32 = 0
        if cidr > 0 {
            mask = UInt32.max << (32 - cidr)
        }
        let b1 = (mask >> 24) & 0xFF
        let b2 = (mask >> 16) & 0xFF
        let b3 = (mask >> 8) & 0xFF
        let b4 = mask & 0xFF
        return "\(b1).\(b2).\(b3).\(b4)"
    }
}
