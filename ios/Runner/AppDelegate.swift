import Flutter
import UIKit
import NetworkExtension
import CryptoKit

@main
@objc class AppDelegate: FlutterAppDelegate {
    private var tunnelChannel: FlutterMethodChannel?
    private var cryptoChannel: FlutterMethodChannel?
    private var vpnManager: NETunnelProviderManager?

    override func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
    ) -> Bool {
        GeneratedPluginRegistrant.register(with: self)

        let controller = window?.rootViewController as! FlutterViewController
        let messenger = controller.binaryMessenger

        // Tunnel channel
        tunnelChannel = FlutterMethodChannel(name: "vpnservice/tunnel", binaryMessenger: messenger)
        tunnelChannel?.setMethodCallHandler(handleTunnelCall)

        // Crypto channel
        cryptoChannel = FlutterMethodChannel(name: "vpnservice/crypto", binaryMessenger: messenger)
        cryptoChannel?.setMethodCallHandler(handleCryptoCall)

        return super.application(application, didFinishLaunchingWithOptions: launchOptions)
    }

    // MARK: - Tunnel Channel

    private func handleTunnelCall(call: FlutterMethodCall, result: @escaping FlutterResult) {
        switch call.method {
        case "connect":
            guard let args = call.arguments as? [String: Any],
                  let privateKey = args["privateKey"] as? String,
                  let serverPublicKey = args["serverPublicKey"] as? String,
                  let serverEndpoint = args["serverEndpoint"] as? String,
                  let serverPort = args["serverPort"] as? Int,
                  let tunnelAddress = args["tunnelAddress"] as? String,
                  let dns = args["dns"] as? String else {
                result(FlutterError(code: "INVALID_ARGS", message: "Missing arguments", details: nil))
                return
            }
            connectVPN(
                privateKey: privateKey,
                serverPublicKey: serverPublicKey,
                serverEndpoint: serverEndpoint,
                serverPort: serverPort,
                tunnelAddress: tunnelAddress,
                dns: dns,
                result: result
            )
        case "disconnect":
            disconnectVPN(result: result)
        case "getStatus":
            getVPNStatus(result: result)
        default:
            result(FlutterMethodNotImplemented)
        }
    }

    private func connectVPN(
        privateKey: String,
        serverPublicKey: String,
        serverEndpoint: String,
        serverPort: Int,
        tunnelAddress: String,
        dns: String,
        result: @escaping FlutterResult
    ) {
        let wgConfig = """
        [Interface]
        PrivateKey = \(privateKey)
        Address = \(tunnelAddress)/32
        DNS = \(dns)

        [Peer]
        PublicKey = \(serverPublicKey)
        AllowedIPs = 0.0.0.0/0, ::/0
        Endpoint = \(serverEndpoint):\(serverPort)
        PersistentKeepalive = 25
        """

        NETunnelProviderManager.loadAllFromPreferences { [weak self] managers, error in
            if let error = error {
                result(FlutterError(code: "LOAD_ERROR", message: error.localizedDescription, details: nil))
                return
            }

            let manager = managers?.first ?? NETunnelProviderManager()
            manager.localizedDescription = "VPN Service"

            let proto = NETunnelProviderProtocol()
            proto.providerBundleIdentifier = "com.vpnservice.vpnservice.tunnel"
            proto.serverAddress = serverEndpoint
            proto.providerConfiguration = ["wgConfig": wgConfig]

            manager.protocolConfiguration = proto
            manager.isEnabled = true

            manager.saveToPreferences { error in
                if let error = error {
                    result(FlutterError(code: "SAVE_ERROR", message: error.localizedDescription, details: nil))
                    return
                }

                manager.loadFromPreferences { error in
                    if let error = error {
                        result(FlutterError(code: "LOAD_ERROR", message: error.localizedDescription, details: nil))
                        return
                    }

                    do {
                        let session = manager.connection as! NETunnelProviderSession
                        try session.startTunnel()
                        self?.vpnManager = manager
                        result(nil)
                    } catch {
                        result(FlutterError(code: "START_ERROR", message: error.localizedDescription, details: nil))
                    }
                }
            }
        }
    }

    private func disconnectVPN(result: @escaping FlutterResult) {
        vpnManager?.connection.stopVPNTunnel()
        result(nil)
    }

    private func getVPNStatus(result: @escaping FlutterResult) {
        guard let manager = vpnManager else {
            result(nil)
            return
        }

        let status = manager.connection.status
        switch status {
        case .connected:
            result(["connected": true, "serverEndpoint": manager.protocolConfiguration?.serverAddress ?? ""])
        default:
            result(nil)
        }
    }

    // MARK: - Crypto Channel

    private func handleCryptoCall(call: FlutterMethodCall, result: @escaping FlutterResult) {
        switch call.method {
        case "generateKeypair":
            generateKeypair(result: result)
        default:
            result(FlutterMethodNotImplemented)
        }
    }

    private func generateKeypair(result: FlutterResult) {
        // Generate 32 random bytes for private key
        var privateKeyBytes = [UInt8](repeating: 0, count: 32)
        let status = SecRandomCopyBytes(kSecRandomDefault, 32, &privateKeyBytes)
        guard status == errSecSuccess else {
            result(FlutterError(code: "KEYGEN_FAILED", message: "Failed to generate random bytes", details: nil))
            return
        }

        // Clamp per RFC 7748
        privateKeyBytes[0] &= 248
        privateKeyBytes[31] &= 127
        privateKeyBytes[31] |= 64

        let privateKeyData = Data(privateKeyBytes)
        let privateKeyBase64 = privateKeyData.base64EncodedString()

        // Derive public key via Curve25519
        let publicKeyBytes = curve25519Multiply(scalar: privateKeyBytes)
        let publicKeyData = Data(publicKeyBytes)
        let publicKeyBase64 = publicKeyData.base64EncodedString()

        result([
            "privateKey": privateKeyBase64,
            "publicKey": publicKeyBase64
        ])
    }

    // X25519 basepoint multiplication
    private func curve25519Multiply(scalar: [UInt8]) -> [UInt8] {
        // Use Apple's CryptoKit on iOS 13+
        if #available(iOS 13.0, *) {
            do {
                let privateKey = try Curve25519.KeyAgreement.PrivateKey(rawRepresentation: Data(scalar))
                return Array(privateKey.publicKey.rawRepresentation)
            } catch {
                return [UInt8](repeating: 0, count: 32)
            }
        }
        return [UInt8](repeating: 0, count: 32)
    }
}
