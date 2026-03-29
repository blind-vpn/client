import 'dart:io';
import 'vpn_service.dart';

/// Desktop implementation of VPN tunnel management.
/// Shells out to wg-quick, same as the CLI client.
class DesktopVpnTunnelService extends VpnTunnelService {
  static const _interface = 'wg-vpn';
  String? _configPath;

  @override
  Future<void> connect({
    required String privateKey,
    required String serverPublicKey,
    required String serverEndpoint,
    required int serverPort,
    required String tunnelAddress,
    required String dns,
  }) async {
    // Write WireGuard config to temp file
    final dir = Directory('${Platform.environment['HOME']}/.vpnservice/wg');
    await dir.create(recursive: true);
    _configPath = '${dir.path}/$_interface.conf';

    final config = '''
[Interface]
PrivateKey = $privateKey
Address = $tunnelAddress/32
DNS = $dns

[Peer]
PublicKey = $serverPublicKey
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = $serverEndpoint:$serverPort
PersistentKeepalive = 25
''';

    await File(_configPath!).writeAsString(config);
    // Set file permissions to 0600 (owner read/write only)
    await Process.run('chmod', ['600', _configPath!]);

    // Bring up tunnel — requires root/sudo
    final result = await Process.run('sudo', ['wg-quick', 'up', _configPath!]);
    if (result.exitCode != 0) {
      throw Exception('wg-quick up failed: ${result.stderr}');
    }
  }

  @override
  Future<void> disconnect() async {
    final result = await Process.run('sudo', ['wg-quick', 'down', _configPath ?? _interface]);
    if (result.exitCode != 0) {
      throw Exception('wg-quick down failed: ${result.stderr}');
    }
  }

  @override
  Future<Map<String, dynamic>?> getStatus() async {
    final result = await Process.run('wg', ['show', _interface]);
    if (result.exitCode != 0) return null;

    final output = result.stdout as String;
    final lines = output.split('\n');
    String? endpoint;
    String? transfer;

    for (final line in lines) {
      final trimmed = line.trim();
      if (trimmed.startsWith('endpoint:')) {
        endpoint = trimmed.split(':').skip(1).join(':').trim();
      }
      if (trimmed.startsWith('transfer:')) {
        transfer = trimmed.split(':').skip(1).join(':').trim();
      }
    }

    return {
      'connected': true,
      'serverEndpoint': endpoint ?? '',
      'transfer': transfer ?? '',
    };
  }
}
