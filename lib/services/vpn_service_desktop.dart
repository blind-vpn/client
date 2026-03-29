import 'dart:io';
import 'vpn_service.dart';

/// Desktop implementation of VPN tunnel management.
///
/// Linux/macOS: shells out to wg-quick.
/// Windows: writes config and invokes the WireGuard CLI (wireguard.exe /installtunnelservice).
class DesktopVpnTunnelService extends VpnTunnelService {
  static const _interface = 'wg-vpn';
  String? _configPath;

  String get _configDir {
    if (Platform.isWindows) {
      final appData = Platform.environment['APPDATA'] ?? Platform.environment['USERPROFILE'] ?? '.';
      return '$appData\\vpnservice\\wg';
    }
    final home = Platform.environment['HOME'] ?? '/tmp';
    return '$home/.vpnservice/wg';
  }

  @override
  Future<void> connect({
    required String privateKey,
    required String serverPublicKey,
    required String serverEndpoint,
    required int serverPort,
    required String tunnelAddress,
    required String dns,
  }) async {
    final dir = Directory(_configDir);
    await dir.create(recursive: true);

    final separator = Platform.isWindows ? '\\' : '/';
    _configPath = '${dir.path}$separator$_interface.conf';

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

    if (Platform.isWindows) {
      await _connectWindows();
    } else {
      await _connectUnix();
    }
  }

  Future<void> _connectWindows() async {
    // WireGuard for Windows: use the CLI to install as a tunnel service.
    // Requires WireGuard to be installed: https://www.wireguard.com/install/
    // The wireguard.exe /installtunnelservice command runs the tunnel as a Windows service.
    final result = await Process.run(
      'wireguard.exe',
      ['/installtunnelservice', _configPath!],
    );
    if (result.exitCode != 0) {
      // Fallback: try using wg-quick via wsl or direct wg commands
      final wgResult = await Process.run('wireguard', ['/installtunnelservice', _configPath!]);
      if (wgResult.exitCode != 0) {
        throw Exception(
          'WireGuard tunnel failed. Is WireGuard installed?\n'
          'Download from: https://www.wireguard.com/install/\n'
          '${result.stderr}',
        );
      }
    }
  }

  Future<void> _connectUnix() async {
    // Set file permissions on Unix only
    await Process.run('chmod', ['600', _configPath!]);
    final result = await Process.run('sudo', ['wg-quick', 'up', _configPath!]);
    if (result.exitCode != 0) {
      throw Exception('wg-quick up failed: ${result.stderr}');
    }
  }

  @override
  Future<void> disconnect() async {
    if (Platform.isWindows) {
      await _disconnectWindows();
    } else {
      await _disconnectUnix();
    }
  }

  Future<void> _disconnectWindows() async {
    final result = await Process.run(
      'wireguard.exe',
      ['/uninstalltunnelservice', _interface],
    );
    if (result.exitCode != 0) {
      await Process.run('wireguard', ['/uninstalltunnelservice', _interface]);
    }
  }

  Future<void> _disconnectUnix() async {
    final result = await Process.run('sudo', ['wg-quick', 'down', _configPath ?? _interface]);
    if (result.exitCode != 0) {
      throw Exception('wg-quick down failed: ${result.stderr}');
    }
  }

  @override
  Future<Map<String, dynamic>?> getStatus() async {
    final wgCmd = Platform.isWindows ? 'wg' : 'wg';
    final result = await Process.run(wgCmd, ['show', _interface]);
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
