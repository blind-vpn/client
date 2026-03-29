import 'package:flutter/foundation.dart';
import '../models/vpn_state.dart';
import 'api_service.dart';
import 'vpn_service.dart';
import 'storage_service.dart';

class VpnProvider extends ChangeNotifier {
  final ApiService api;
  final VpnTunnelService tunnel;
  final StorageService storage;

  ConnectionStatus _status = ConnectionStatus.disconnected;
  List<ServerInfo> _servers = [];
  ServerInfo? _selectedServer;
  String? _error;
  DateTime? _connectedSince;

  VpnProvider({
    required this.api,
    required this.tunnel,
    required this.storage,
  });

  ConnectionStatus get status => _status;
  List<ServerInfo> get servers => _servers;
  ServerInfo? get selectedServer => _selectedServer;
  String? get error => _error;
  Duration? get connectedDuration => _connectedSince != null
      ? DateTime.now().difference(_connectedSince!)
      : null;
  bool get isLoggedIn => storage.isLoggedIn;

  Future<void> init() async {
    await storage.init();
    if (storage.isLoggedIn) {
      api.accountID = storage.accountID;
      await refreshServers();
      // Restore selected server
      if (storage.selectedServerID != null) {
        _selectedServer = _servers.where((s) => s.id == storage.selectedServerID).firstOrNull;
      }
    }
  }

  Future<void> createAccount() async {
    try {
      _error = null;
      final resp = await api.createAccount();
      storage.accountID = resp['account_id'] as String;
      api.accountID = storage.accountID;

      // Generate WireGuard keypair
      await storage.generateAndStoreKeypair();

      await refreshServers();
      notifyListeners();
    } catch (e) {
      _error = e.toString();
      notifyListeners();
    }
  }

  Future<void> refreshServers() async {
    try {
      _servers = await api.listServers();
      _error = null;
      notifyListeners();
    } catch (e) {
      _error = e.toString();
      notifyListeners();
    }
  }

  void selectServer(ServerInfo server) {
    _selectedServer = server;
    storage.selectedServerID = server.id;
    notifyListeners();
  }

  Future<void> connect() async {
    if (_selectedServer == null) {
      _error = 'No server selected';
      notifyListeners();
      return;
    }

    try {
      _status = ConnectionStatus.connecting;
      _error = null;
      notifyListeners();

      // Register key with API
      final pubKey = storage.publicKey;
      if (pubKey == null) {
        _error = 'No keypair generated';
        _status = ConnectionStatus.disconnected;
        notifyListeners();
        return;
      }

      KeyRegistration reg;
      try {
        reg = await api.registerKey(pubKey);
      } on ApiException catch (e) {
        if (e.message.contains('duplicate') || e.message.contains('unique')) {
          // Key already registered, get existing assignment
          final keys = await api.listKeys();
          if (keys.isEmpty) {
            throw Exception('No keys registered');
          }
          final key = keys.first;
          reg = KeyRegistration(
            id: key['id'] as String,
            serverID: key['server_id'] as String,
            serverIP: _selectedServer!.publicIP,
            serverPubkey: _selectedServer!.wgPubkey,
            serverPort: _selectedServer!.wgPort,
            allowedIP: key['allowed_ip'] as String,
          );
        } else {
          rethrow;
        }
      }

      // Connect via platform tunnel
      await tunnel.connect(
        privateKey: storage.privateKey!,
        serverPublicKey: reg.serverPubkey,
        serverEndpoint: reg.serverIP,
        serverPort: reg.serverPort,
        tunnelAddress: reg.allowedIP.replaceAll('/32', ''),
        dns: reg.serverIP,
      );

      _status = ConnectionStatus.connected;
      _connectedSince = DateTime.now();
      notifyListeners();
    } catch (e) {
      _error = e.toString();
      _status = ConnectionStatus.disconnected;
      notifyListeners();
    }
  }

  Future<void> disconnect() async {
    try {
      _status = ConnectionStatus.disconnecting;
      notifyListeners();

      await tunnel.disconnect();

      _status = ConnectionStatus.disconnected;
      _connectedSince = null;
      notifyListeners();
    } catch (e) {
      _error = e.toString();
      _status = ConnectionStatus.disconnected;
      notifyListeners();
    }
  }
}
