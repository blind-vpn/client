import 'package:flutter/services.dart';

/// Platform channel bridge to native WireGuard tunnel implementation.
///
/// On mobile (Android/iOS), this calls into platform-specific VPN APIs:
///   - Android: VpnService + wireguard-android
///   - iOS: NetworkExtension + WireGuardKit
///
/// On desktop (Windows/Mac/Linux), this shells out to wg-quick.
class VpnTunnelService {
  static const _channel = MethodChannel('vpnservice/tunnel');

  Future<void> connect({
    required String privateKey,
    required String serverPublicKey,
    required String serverEndpoint,
    required int serverPort,
    required String tunnelAddress,
    required String dns,
  }) async {
    await _channel.invokeMethod('connect', {
      'privateKey': privateKey,
      'serverPublicKey': serverPublicKey,
      'serverEndpoint': serverEndpoint,
      'serverPort': serverPort,
      'tunnelAddress': tunnelAddress,
      'dns': dns,
    });
  }

  Future<void> disconnect() async {
    await _channel.invokeMethod('disconnect');
  }

  Future<Map<String, dynamic>?> getStatus() async {
    final result = await _channel.invokeMethod('getStatus');
    if (result == null) return null;
    return Map<String, dynamic>.from(result as Map);
  }
}
